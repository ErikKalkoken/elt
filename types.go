package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"slices"
	"strconv"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/urfave/cli/v3"
)

type Type struct {
	Description string `json:"description,omitempty"`
	GroupID     int32  `json:"group_id,omitempty"`
	Name        string `json:"name,omitempty"`
	TypeID      int32  `json:"type_id"`
	Error       string `json:"error,omitempty"`
}

func types(ctx context.Context, cmd *cli.Command) error {
	if err := setLogLevel(cmd); err != nil {
		return err
	}
	client := retryablehttp.NewClient()
	client.Logger = slog.Default()
	items := make([]Type, 0)
	for _, id := range cmd.Int32Args("ID") {
		res, err := client.Get("https://" + path.Join(esiBaseURL, "universe", "types", strconv.Itoa(int(id))))
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			items = append(items, Type{
				TypeID: id,
				Error:  "Not found",
			})
			continue
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("API returned error: %s", res.Status)
		}
		var item Type
		if err := json.NewDecoder(res.Body).Decode(&item); err != nil {
			return err
		}
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b Type) int {
		return cmp.Compare(a.TypeID, b.TypeID)
	})
	out, err := json.MarshalIndent(items, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
