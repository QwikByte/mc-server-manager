// Package app wires the master together and provides its command line interface.
package app

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
	"github.com/QwikByte/mc-server-manager/internal/logging"
	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/auth"
	"github.com/QwikByte/mc-server-manager/internal/master/database"
	"github.com/QwikByte/mc-server-manager/internal/master/logs"
)

type config struct {
	dataDir    string
	httpAddr   string
	tlsCert    string
	tlsKey     string
	enrollAddr string
	publicAddr string
	log        logging.Options
}

// Command returns the root command of mcsm-master.
func Command() *cobra.Command {
	var cfg config
	root := &cobra.Command{
		Use:          "mcsm-master",
		Short:        "Admin panel and control plane of MC Server Manager",
		Version:      buildinfo.Version,
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&cfg.dataDir, "data-dir", "/var/lib/mcsm-master", "directory for the database and the certificate authority")

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Run the admin panel and the enrollment endpoint for agents",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return serve(cmd.Context(), cfg) },
	}
	f := serve.Flags()
	f.StringVar(&cfg.httpAddr, "http-addr", "127.0.0.1:8080", "listen address of the admin panel")
	f.StringVar(&cfg.tlsCert, "tls-cert", "", "TLS certificate of the admin panel; omit behind a TLS-terminating reverse proxy")
	f.StringVar(&cfg.tlsKey, "tls-key", "", "TLS private key of the admin panel")
	f.StringVar(&cfg.enrollAddr, "enroll-addr", ":9443", "listen address of the enrollment endpoint")
	f.StringVar(&cfg.publicAddr, "public-enroll-addr", "", "host:port agents use to reach the enrollment endpoint (default <hostname>:<enroll port>); the panel's settings can replace it")
	cfg.log.AddFlags(f)

	user := &cobra.Command{Use: "user", Short: "Manage administrator accounts"}
	user.AddCommand(&cobra.Command{
		Use:   "add <username>",
		Short: "Create an administrator account (reads the password from the terminal or stdin), e.g. the first one or after losing access",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return addUser(cmd.Context(), cfg, args[0]) },
	})

	// The log on the master's host shows everything, like the panel to administrators.
	logsCmd := logs.Command(func() (*logs.Store, error) {
		db, err := openDB(cfg.dataDir)
		return logs.NewStore(db, nil, nil), err
	})
	logsCmd.PreRun = func(cmd *cobra.Command, _ []string) {
		cmd.SetContext(access.WithGrants(cmd.Context(), access.Admin()))
	}

	root.AddCommand(serve, user, logsCmd)
	return root
}

func openDB(dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	return database.Open(filepath.Join(dataDir, "master.db"))
}

func addUser(ctx context.Context, cfg config, username string) error {
	password, err := readPassword()
	if err != nil {
		return err
	}
	db, err := openDB(cfg.dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	user, err := auth.NewService(db).CreateUser(ctx, username, password)
	if err == nil {
		err = access.NewService(db).MakeAdmin(ctx, user.ID)
	}
	if err == nil {
		fmt.Printf("Created administrator %q. Invite further users in the panel.\n", username)
	}
	return err
}

// readPassword prompts twice on a terminal and reads a single line from piped stdin.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "\nRepeat password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}
