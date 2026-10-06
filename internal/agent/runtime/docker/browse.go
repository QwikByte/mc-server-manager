package docker

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

const (
	// maxValue is how many characters of a value a page shows.
	maxValue = 200
	// maxPage is how many bytes the rows of a page may have, below the 4 MiB that gRPC accepts
	// by default.
	maxPage = 3 << 20
	// browseTimeout limits each statement of a browsing session, in seconds.
	browseTimeout = 10
)

// column is a column of a table as an engine describes it.
type column struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"` // MariaDB's data type, e.g. blob
	Key  string `json:"key"`  // PRI for one of the primary key
}

// browser is how the agent reads the tables of an engine's databases: as the superuser in a
// session that only reads, with statements it builds itself from names that need no escaping,
// as noryxv1.DatabaseName and noryxv1.TableName match them. Each statement prints a JSON value
// per line.
type browser struct {
	// client reads SQL from its standard input on a database, with the superuser's environment.
	client  func(database string) []string
	session string
	// tables prints a noryxv1.Table per line, columns a column.
	tables  func(database string) string
	columns func(database, schema, table string) string
	// cell is a column's value as text of at most maxValue+1 characters.
	cell func(c column) string
	// rows prints an array of the cells of each row.
	rows func(schema, table string, cells, order []string, offset uint64, limit uint32) string
}

// mariadbBinary are the data types of MariaDB without a character set, which a page shows as
// hexadecimal numbers.
var mariadbBinary = []string{"binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob", "bit", "geometry", "point",
	"linestring", "polygon", "multipoint", "multilinestring", "multipolygon", "geometrycollection"}

var browsers = map[noryxv1.DatastoreEngine]browser{
	noryxv1.DatastoreEngine_DATASTORE_ENGINE_MARIADB: {
		// The sandbox refuses the commands of the client itself, and raw output keeps JSON as it is.
		client: func(db string) []string {
			return []string{"mariadb", "-uroot", "-N", "-B", "-r", "--sandbox", "--default-character-set=utf8mb4", db}
		},
		session: fmt.Sprintf("SET SESSION TRANSACTION READ ONLY; SET SESSION max_statement_time = %d;\n", browseTimeout),
		tables: func(db string) string {
			return fmt.Sprintf("SELECT JSON_OBJECT('name', TABLE_NAME, 'rows', IFNULL(TABLE_ROWS, -1), 'size', IFNULL(DATA_LENGTH + INDEX_LENGTH, 0))"+
				" FROM information_schema.TABLES WHERE TABLE_SCHEMA = '%s' AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME;", db)
		},
		columns: func(db, _, table string) string {
			return fmt.Sprintf("SELECT JSON_OBJECT('name', COLUMN_NAME, 'type', COLUMN_TYPE, 'data', DATA_TYPE, 'key', COLUMN_KEY)"+
				" FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s' ORDER BY ORDINAL_POSITION;", db, table)
		},
		cell: func(c column) string {
			if slices.Contains(mariadbBinary, c.Data) {
				return fmt.Sprintf("CONCAT('0x', HEX(LEFT(`%s`, %d)))", c.Name, (maxValue-2)/2+1)
			}
			return fmt.Sprintf("LEFT(`%s`, %d)", c.Name, maxValue+1)
		},
		rows: func(_, table string, cells, order []string, offset uint64, limit uint32) string {
			return fmt.Sprintf("SELECT JSON_ARRAY(%s) FROM `%s`%s LIMIT %d OFFSET %d;", strings.Join(cells, ", "), table, orderBy(order, "`"), limit, offset)
		},
	},
	noryxv1.DatastoreEngine_DATASTORE_ENGINE_POSTGRES: {
		client:  func(db string) []string { return slices.Concat(psql, []string{"-A", "-t", "-U", "postgres", "-d", db}) },
		session: fmt.Sprintf("SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY; SET statement_timeout = '%ds';\n", browseTimeout),
		tables: func(string) string {
			return "SELECT json_build_object('schema', n.nspname, 'name', c.relname, 'rows', c.reltuples::bigint, 'size', pg_total_relation_size(c.oid))" +
				" FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace" +
				" WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition AND n.nspname <> 'information_schema' AND n.nspname !~ '^pg_' ORDER BY n.nspname, c.relname;"
		},
		columns: func(_, schema, table string) string {
			return fmt.Sprintf("SELECT json_build_object('name', a.attname, 'type', format_type(a.atttypid, a.atttypmod), 'key', CASE WHEN a.attnum = ANY(i.indkey) THEN 'PRI' ELSE '' END)"+
				" FROM pg_attribute a LEFT JOIN pg_index i ON i.indrelid = a.attrelid AND i.indisprimary"+
				" WHERE a.attrelid = to_regclass('\"%s\".\"%s\"') AND a.attnum > 0 AND NOT a.attisdropped ORDER BY a.attnum;", schema, table)
		},
		cell: func(c column) string { return fmt.Sprintf(`left(%q::text, %d)`, c.Name, maxValue+1) },
		rows: func(schema, table string, cells, order []string, offset uint64, limit uint32) string {
			return fmt.Sprintf(`SELECT to_json(ARRAY[%s]::text[]) FROM %q.%q%s LIMIT %d OFFSET %d;`, strings.Join(cells, ", "), schema, table, orderBy(order, `"`), limit, offset)
		},
	},
}

func orderBy(columns []string, quote string) string {
	if len(columns) == 0 {
		return ""
	}
	return " ORDER BY " + quote + strings.Join(columns, quote+", "+quote) + quote
}

// browse runs a statement that sql builds on a database of a ready datastore, in a session
// that only reads, and decodes each line it prints with decode.
func (d *Docker) browse(ctx context.Context, id, name string, sql func(b browser) string, decode func(line []byte) error) error {
	s, err := d.session(ctx, id, name)
	if err != nil {
		return err
	}
	b := browsers[s.engine]
	_, env := s.dialect.superuser(s.password)
	out := &capped{max: maxPage}
	if err := d.exec(ctx, id, b.client(name), env, strings.NewReader(b.session+sql(b)), out, s.password); err != nil {
		return err
	}
	lines := bufio.NewScanner(&out.Buffer)
	lines.Buffer(nil, maxPage)
	for lines.Scan() {
		if err := decode(lines.Bytes()); err != nil {
			return fmt.Errorf("unexpected output: %w", err)
		}
	}
	return lines.Err()
}

var errTooLarge = fmt.Errorf("the rows are larger than %d MiB", maxPage>>20)

// capped keeps what is written to it, up to max bytes.
type capped struct {
	bytes.Buffer
	max int
}

func (c *capped) Write(p []byte) (int, error) {
	if c.Len()+len(p) > c.max {
		return 0, errTooLarge
	}
	return c.Buffer.Write(p)
}

func (d *Docker) Tables(ctx context.Context, id, name string) ([]*noryxv1.Table, error) {
	tables := []*noryxv1.Table{}
	err := d.browse(ctx, id, name, func(b browser) string { return b.tables(name) }, func(line []byte) error {
		t := &noryxv1.Table{}
		tables = append(tables, t)
		return protojson.Unmarshal(line, t)
	})
	return tables, err
}

func (d *Docker) Browse(ctx context.Context, id, name, schema, table string, offset uint64, limit uint32) (*noryxv1.BrowseTableResponse, error) {
	schema = cmp.Or(schema, "public") // MariaDB has none
	if !noryxv1.TableName.MatchString(table) || !noryxv1.TableName.MatchString(schema) {
		return nil, runtime.ErrNoTable
	}
	var columns []column
	err := d.browse(ctx, id, name, func(b browser) string { return b.columns(name, schema, table) }, func(line []byte) error {
		var c column
		if err := json.Unmarshal(line, &c); err != nil {
			return err
		}
		if !noryxv1.TableName.MatchString(c.Name) {
			return fmt.Errorf("noryx only shows columns whose names have letters, digits, _, $ and -, unlike %q", c.Name)
		}
		columns = append(columns, c)
		return nil
	})
	switch {
	case err != nil:
		return nil, err
	case len(columns) == 0:
		return nil, runtime.ErrNoTable
	}
	res := &noryxv1.BrowseTableResponse{}
	var order []string
	for _, c := range columns {
		res.Columns = append(res.Columns, &noryxv1.TableColumn{Name: c.Name, Type: c.Type, PrimaryKey: c.Key == "PRI"})
		if c.Key == "PRI" {
			order = append(order, c.Name)
		}
	}
	rows := func(b browser) string {
		cells := make([]string, len(columns))
		for i, c := range columns {
			cells[i] = b.cell(c)
		}
		return b.rows(schema, table, cells, order, offset, limit+1)
	}
	err = d.browse(ctx, id, name, rows, func(line []byte) error {
		var values []*string
		if err := json.Unmarshal(line, &values); err != nil {
			return err
		}
		if len(values) != len(columns) {
			return errors.New("a row doesn't fit the columns")
		}
		row := &noryxv1.TableRow{}
		for _, v := range values {
			row.Values = append(row.Values, value(v))
		}
		res.Rows = append(res.Rows, row)
		return nil
	})
	if len(res.Rows) > int(limit) {
		res.Rows, res.More = res.Rows[:limit], true
	}
	return res, err
}

// value cuts text to maxValue characters.
func value(text *string) *noryxv1.TableValue {
	if text == nil {
		return &noryxv1.TableValue{Null: true}
	}
	if r := []rune(*text); len(r) > maxValue {
		return &noryxv1.TableValue{Text: string(r[:maxValue]), Truncated: true}
	}
	return &noryxv1.TableValue{Text: *text}
}
