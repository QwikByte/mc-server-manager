package datastore

import (
	"context"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// pageRows is how many rows of a table a page has.
const pageRows = 50

// Table is a table of a database.
type Table struct {
	Schema string `json:"schema,omitempty"` // of PostgreSQL
	Name   string `json:"name"`
	Rows   int64  `json:"rows"` // estimated; -1 if unknown
	Size   int64  `json:"size"`
}

// Page is rows of a table as text, with long values cut short.
type Page struct {
	Columns []Column  `json:"columns"`
	Rows    [][]Value `json:"rows"`
	More    bool      `json:"more"`
}

type Column struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	PrimaryKey bool   `json:"primaryKey,omitempty"`
}

type Value struct {
	Text      string `json:"text"`
	Null      bool   `json:"null,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Tables returns the tables of a database.
func (s *Service) Tables(ctx context.Context, id, name string) ([]Table, error) {
	ds, _, err := s.database(ctx, id, name)
	if err != nil {
		return nil, err
	}
	tables := []Table{}
	err = s.call(ctx, ds.NodeID, func(ctx context.Context, c noryxv1.DatastoreServiceClient) error {
		res, err := c.ListTables(ctx, &noryxv1.ListTablesRequest{Id: id, Database: name})
		for _, t := range res.GetTables() {
			tables = append(tables, Table{t.GetSchema(), t.GetName(), t.GetRows(), t.GetSize()})
		}
		return err
	})
	return tables, browsing(err)
}

// Browse returns a page of the rows of a table, which the agent only reads.
func (s *Service) Browse(ctx context.Context, id, name, schema, table string, offset uint64) (Page, error) {
	ds, _, err := s.database(ctx, id, name)
	if err != nil {
		return Page{}, err
	}
	page := Page{Columns: []Column{}, Rows: [][]Value{}}
	err = s.call(ctx, ds.NodeID, func(ctx context.Context, c noryxv1.DatastoreServiceClient) error {
		res, err := c.BrowseTable(ctx, &noryxv1.BrowseTableRequest{Id: id, Database: name, Schema: schema, Table: table, Offset: offset, Limit: pageRows})
		for _, col := range res.GetColumns() {
			page.Columns = append(page.Columns, Column{col.GetName(), col.GetType(), col.GetPrimaryKey()})
		}
		for _, row := range res.GetRows() {
			values := make([]Value, len(row.GetValues()))
			for i, v := range row.GetValues() {
				values[i] = Value{v.GetText(), v.GetNull(), v.GetTruncated()}
			}
			page.Rows = append(page.Rows, values)
		}
		page.More = res.GetMore()
		return err
	})
	return page, browsing(err)
}

// browsing tells to update agents that can't browse databases yet.
func browsing(err error) error {
	if status.Code(err) == codes.Unimplemented {
		return httpapi.Errorf(http.StatusNotImplemented, "Update the agent of the datastore's node to look into its databases.")
	}
	return err
}
