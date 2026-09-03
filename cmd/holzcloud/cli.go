package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/config"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// usage is printed for `holzcloud help` and on an unknown subcommand.
const usage = `holzcloud — self-hosted CMS

Usage:
  holzcloud [serve]                 run the HTTP server (default)
  holzcloud version                 print version and commit
  holzcloud user list               list accounts
  holzcloud user create             create an account
  holzcloud user passwd             set a password
  holzcloud user 2fa status         show who has a second factor
  holzcloud user 2fa disable        remove a second factor (the way back in)
  holzcloud backup <dir>            write a verified database snapshot
  holzcloud migrate status|up       show or apply pending migrations
  holzcloud compact                 rebuild the database file (VACUUM)
  holzcloud rerender                re-render every page from its Markdown
  holzcloud thumbnails              generate the scaled copies of older images
  holzcloud check                   run an integrity check
  holzcloud template check <path>   check a template directory or .zip
  holzcloud template spec           print the template authoring specification

Passwords are read from stdin so they never reach the shell history:
  echo -n 'new password' | holzcloud user passwd -email admin@example.com

Every subcommand honours the same HOLZCLOUD_* environment variables as the
server. See deploy/DEPLOY.md for the recovery procedure.
`

// runCLI dispatches a subcommand. It returns false when args carry no
// subcommand, in which case the caller starts the server.
func runCLI(args []string) (handled bool, err error) {
	if len(args) < 2 {
		return false, nil
	}
	switch args[1] {
	case "serve":
		return false, nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return true, nil
	case "version", "-version", "--version":
		fmt.Printf("holzcloud %s (%s)\n", Version, Commit)
		return true, nil
	case "user":
		return true, cmdUser(args[2:])
	case "backup":
		return true, cmdBackup(args[2:])
	case "migrate":
		return true, cmdMigrate(args[2:])
	case "compact":
		return true, cmdCompact(args[2:])
	case "rerender":
		return true, cmdRerender(args[2:])
	case "thumbnails":
		return true, cmdThumbnails(args[2:])
	case "check":
		return true, cmdCheck(args[2:])
	case "template":
		return true, cmdTemplate(args[2:])
	default:
		if strings.HasPrefix(args[1], "-") {
			// A bare flag is meant for the server, not a subcommand.
			return false, nil
		}
		fmt.Fprint(os.Stderr, usage)
		return true, fmt.Errorf("unknown command %q", args[1])
	}
}

// openForCLI loads the configuration and opens the database with migrations
// applied, which is what every subcommand needs and nothing more — no HTTP
// server, no session manager.
func openForCLI() (config.Config, *db.DB, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, nil, err
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return cfg, nil, fmt.Errorf("open database at %s: %w", cfg.DBPath, err)
	}
	if err := db.RunMigrations(database.Write); err != nil {
		database.Close()
		return cfg, nil, fmt.Errorf("migrations: %w", err)
	}
	return cfg, database, nil
}

func argon2ParamsFor(cfg config.Config) auth.Argon2Params {
	return auth.Argon2Params{
		Memory:      cfg.Argon2Memory,
		Iterations:  cfg.Argon2Iterations,
		Parallelism: cfg.Argon2Parallelism,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// readPassword takes the password from stdin, so it never appears in the shell
// history or in the process list.
func readPassword(in io.Reader) (string, error) {
	data, err := io.ReadAll(bufio.NewReader(in))
	if err != nil {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	pw := strings.TrimRight(string(data), "\r\n")
	if pw == "" {
		return "", errors.New("no password on stdin — pipe it in, e.g. echo -n 'secret' | holzcloud user passwd -email ...")
	}
	return pw, nil
}

func cmdUser(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: holzcloud user list|create|passwd|2fa")
	}

	cfg, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()

	store := user.NewStore(database, argon2ParamsFor(cfg))
	ctx := context.Background()

	switch args[0] {
	case "2fa":
		return cmdUserTwoFactor(ctx, store, args[1:])
	case "list":
		users, err := store.List(ctx)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tROLE\tEMAIL\tNAME\tCREATED")
		for _, u := range users {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", u.ID, u.Role, u.Email, u.Name, u.CreatedAt)
		}
		return w.Flush()

	case "create":
		fs := flag.NewFlagSet("user create", flag.ContinueOnError)
		email := fs.String("email", "", "email address")
		name := fs.String("name", "", "display name")
		role := fs.String("role", user.RoleAdmin, "admin or editor")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *email == "" {
			return errors.New("-email is required")
		}
		pw, err := readPassword(os.Stdin)
		if err != nil {
			return err
		}
		id, err := store.Create(ctx, *name, *email, pw, *role)
		if err != nil {
			return err
		}
		fmt.Printf("created user %d (%s, %s)\n", id, *email, *role)
		return nil

	case "passwd":
		fs := flag.NewFlagSet("user passwd", flag.ContinueOnError)
		email := fs.String("email", "", "email address of the account")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *email == "" {
			return errors.New("-email is required")
		}
		u, err := store.GetByEmail(ctx, *email)
		if err != nil {
			return err
		}
		if u == nil {
			return fmt.Errorf("no account with email %q", *email)
		}
		pw, err := readPassword(os.Stdin)
		if err != nil {
			return err
		}
		if err := store.SetPassword(ctx, u.ID, pw); err != nil {
			return err
		}
		fmt.Printf("password updated for %s (id %d)\n", u.Email, u.ID)
		fmt.Println("note: existing sessions of this account stay valid until they expire; " +
			"restart the service to drop them all")
		return nil

	default:
		return fmt.Errorf("unknown user command %q", args[0])
	}
}

func cmdBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("usage: holzcloud backup <target-file-or-directory>")
	}

	_, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()

	target, err := db.Backup(context.Background(), database, fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Printf("backup written and verified: %s\n", target)
	return nil
}

func cmdCheck(args []string) error {
	_, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()

	result, err := db.QuickCheck(context.Background(), database.Read)
	if err != nil {
		return err
	}
	fmt.Printf("integrity: %s\n", result)
	if result != "ok" {
		return errors.New("database reports corruption — restore from a backup")
	}
	return nil
}

func cmdCompact(args []string) error {
	_, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()

	before, after, err := db.Compact(context.Background(), database)
	if err != nil {
		return err
	}
	fmt.Printf("compacted: %d -> %d bytes (%d reclaimed)\n", before, after, before-after)
	return nil
}

func cmdMigrate(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: holzcloud migrate status|up")
	}

	_, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()

	switch args[0] {
	case "status":
		// openForCLI already applied everything, so reaching here means current.
		version, err := db.CurrentVersion(database.Write)
		if err != nil {
			return err
		}
		fmt.Printf("schema version: %d (up to date)\n", version)
		return nil
	case "up":
		version, err := db.CurrentVersion(database.Write)
		if err != nil {
			return err
		}
		fmt.Printf("migrations applied, schema version: %d\n", version)
		return nil
	default:
		return fmt.Errorf("unknown migrate command %q", args[0])
	}
}

// cmdRerender re-renders every page from its stored Markdown.
//
// Needed whenever the rendering pipeline changes: enabling raw HTML in goldmark
// only affects pages saved afterwards, so without this an existing page keeps
// the empty body the old renderer produced.
func cmdRerender(args []string) error {
	fs := flag.NewFlagSet("rerender", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "report what would change without writing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	_, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	rows, err := database.Read.QueryContext(ctx,
		`SELECT id, website_id, slug, content_markdown, content_html FROM pages ORDER BY id`)
	if err != nil {
		return fmt.Errorf("list pages: %w", err)
	}

	type change struct {
		id        int64
		websiteID int64
		slug      string
		html      string
	}
	var changes []change
	var total int

	for rows.Next() {
		var id, websiteID int64
		var slug, markdown, storedHTML string
		if err := rows.Scan(&id, &websiteID, &slug, &markdown, &storedHTML); err != nil {
			rows.Close()
			return fmt.Errorf("scan page: %w", err)
		}
		total++
		rendered, err := page.RenderMarkdown(markdown)
		if err != nil {
			rows.Close()
			return fmt.Errorf("render page %d: %w", id, err)
		}
		if rendered != storedHTML {
			changes = append(changes, change{id, websiteID, slug, rendered})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	fmt.Printf("%d pages, %d would change\n", total, len(changes))
	for _, c := range changes {
		fmt.Printf("  site %d  /%s (id %d)\n", c.websiteID, c.slug, c.id)
	}
	if *dryRun || len(changes) == 0 {
		return nil
	}

	tx, err := database.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()
	for _, c := range changes {
		if _, err := tx.ExecContext(ctx,
			`UPDATE pages SET content_html = $1 WHERE id = $2`, c.html, c.id); err != nil {
			return fmt.Errorf("update page %d: %w", c.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	fmt.Printf("re-rendered %d pages\n", len(changes))
	return nil
}

// cmdUserTwoFactor is the way back in when a phone is gone.
//
// Two-factor is compulsory for administrators, which makes the phone a single
// point of failure for the whole installation unless there is a door that does
// not go through it. Recovery codes are the first answer; this is the second,
// for when those have run out too.
//
// It deliberately requires shell access to the machine — the one thing an
// attacker who has the password but not the phone does not have.
func cmdUserTwoFactor(ctx context.Context, store *user.Store, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: holzcloud user 2fa status|disable -email <address>")
	}

	fs := flag.NewFlagSet("2fa", flag.ContinueOnError)
	email := fs.String("email", "", "account address")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	switch args[0] {
	case "status":
		users, err := store.List(ctx)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tROLE\tEMAIL\tSECOND FACTOR\tRECOVERY CODES LEFT")
		for _, u := range users {
			if *email != "" && !strings.EqualFold(u.Email, *email) {
				continue
			}
			tf, err := store.GetTwoFactor(ctx, u.ID)
			if err != nil {
				return err
			}
			state, left := "off", 0
			if tf != nil && tf.Enabled() {
				state, left = "on", tf.RecoveryLeft
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n", u.ID, u.Role, u.Email, state, left)
		}
		return w.Flush()

	case "disable":
		if *email == "" {
			return errors.New("usage: holzcloud user 2fa disable -email <address>")
		}
		u, err := store.GetByEmail(ctx, *email)
		if err != nil {
			return err
		}
		if u == nil {
			return fmt.Errorf("no account with address %q", *email)
		}
		if err := store.DisableTwoFactor(ctx, u.ID); err != nil {
			return err
		}
		// Saying what happens next matters: the account can sign in with its
		// password alone right now, and for an administrator the next sign-in
		// will insist on setting a new authenticator up.
		fmt.Printf("Second factor removed for %s.\n", u.Email)
		fmt.Println("The account can now sign in with its password alone.")
		if u.Role == "admin" {
			fmt.Println("As an administrator it will be asked to set up a new authenticator immediately.")
		}
		return nil
	}
	return fmt.Errorf("unknown 2fa command %q", args[0])
}

// cmdThumbnails generates the scaled copies of images uploaded before the
// pipeline existed.
//
// Without it, an upgrade leaves every existing photo full-size forever: the
// variants are made once, at upload, and nothing else ever revisits a file.
func cmdThumbnails(args []string) error {
	fs := flag.NewFlagSet("thumbnails", flag.ContinueOnError)
	force := fs.Bool("force", false, "also regenerate images that already have copies")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	where := `WHERE width = 0`
	if *force {
		where = ""
	}
	rows, err := database.Read.QueryContext(ctx,
		`SELECT id, website_id, filename, mime_type FROM media `+where+` ORDER BY id`)
	if err != nil {
		return fmt.Errorf("list media: %w", err)
	}

	type item struct {
		id        int64
		websiteID int64
		filename  string
		mimeType  string
	}
	var todo []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.websiteID, &it.filename, &it.mimeType); err != nil {
			rows.Close()
			return fmt.Errorf("scan media: %w", err)
		}
		if media.CanMakeVariants(it.mimeType) {
			todo = append(todo, it)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	store := media.NewStore(database)
	var done, failed int
	for _, it := range todo {
		dir := filepath.Join(cfg.DataDir, "media", strconv.FormatInt(it.websiteID, 10))
		source := filepath.Join(dir, it.filename)

		variants, err := media.MakeVariants(source, dir, it.filename, it.mimeType, cfg.MaxMegapixels)
		if err != nil {
			// One unreadable or oversized file must not stop the run: the point
			// is to get through the whole library.
			fmt.Fprintf(os.Stderr, "  %s: %v\n", it.filename, err)
			failed++
			continue
		}
		width, height, err := media.Dimensions(source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", it.filename, err)
			failed++
			continue
		}
		if err := store.SaveVariants(ctx, it.id, width, height, variants); err != nil {
			return fmt.Errorf("save variants for %s: %w", it.filename, err)
		}
		done++
	}

	fmt.Printf("%d images processed, %d failed\n", done, failed)
	return nil
}

// parseID is a small helper for subcommands taking a numeric argument.
func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%q is not a valid id", s)
	}
	return id, nil
}
