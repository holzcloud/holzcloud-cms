package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
)

// cmdAI manages the keys an AI assistant connects with.
//
// This is the way in without the web interface: whoever can run the binary on
// the server can hand an assistant a key, and from there the assistant can do
// everything the admin can. An admin key — one that may also manage users,
// plugins and further keys — is only ever created here, never through the
// connection itself and never on a screen: whoever holds the server holds the
// installation anyway, and nobody else should be able to mint one.
func cmdAI(args []string) error {
	if len(args) < 2 || args[0] != "key" {
		return errors.New("usage: holzcloud ai key create|list|revoke")
	}
	_, database, err := openForCLI()
	if err != nil {
		return err
	}
	defer database.Close()
	store := ai.NewStore(database)
	ctx := context.Background()

	switch args[1] {
	case "create":
		fs := flag.NewFlagSet("ai key create", flag.ContinueOnError)
		name := fs.String("name", "", "what the key is for, e.g. \"Claude on my laptop\"")
		level := fs.String("level", "content", "read, content or admin")
		website := fs.Int64("website", 0, "limit the key to one website (not for admin keys)")
		days := fs.Int("days", 0, "expire after this many days; 0 never expires")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		lvl, err := ai.ParseLevel(*level)
		if err != nil {
			return err
		}
		secret, tok, err := store.IssueLevel(ctx, *name, *website, lvl, time.Duration(*days)*24*time.Hour)
		if errors.Is(err, ai.ErrNameMissing) {
			return errors.New("give the key a name: -name \"what it is for\"")
		}
		if err != nil {
			return err
		}
		fmt.Printf("key %d created: %s, level %s\n\n", tok.ID, tok.Name, tok.LevelOf())
		fmt.Println(secret)
		fmt.Println("\nThis is the only time the key is shown. Connect an assistant to /ai on this")
		fmt.Println("server with the header:  Authorization: Bearer <the key above>")
		return nil

	case "list":
		keys, err := store.List(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tLEVEL\tWEBSITE\tLAST USED\tEXPIRES")
		for _, k := range keys {
			site := "all"
			if k.WebsiteID != 0 {
				site = strconv.FormatInt(k.WebsiteID, 10)
			}
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n", k.ID, k.Name, k.LevelOf(), site, when(k.LastUsedAt), when(k.ExpiresAt))
		}
		return tw.Flush()

	case "revoke":
		if len(args) < 3 {
			return errors.New("usage: holzcloud ai key revoke <id>")
		}
		id, err := strconv.ParseInt(strings.TrimSpace(args[2]), 10, 64)
		if err != nil {
			return fmt.Errorf("not a key id: %q", args[2])
		}
		if err := store.Revoke(ctx, id); err != nil {
			return err
		}
		fmt.Printf("key %d revoked\n", id)
		return nil
	}
	return errors.New("usage: holzcloud ai key create|list|revoke")
}

func when(t *time.Time) string {
	if t == nil {
		return "–"
	}
	return t.Format("2006-01-02 15:04")
}
