# everef

**everef** is a command line tool for looking up Eve Online objects.

[![GitHub Release](https://img.shields.io/github/v/release/ErikKalkoken/everef)](https://github.com/ErikKalkoken/everef/everef)
[![build status](https://github.com/ErikKalkoken/everef/actions/workflows/go.yml/badge.svg)](https://github.com/ErikKalkoken/everef/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ErikKalkoken/everef)](https://goreportcard.com/report/github.com/ErikKalkoken/everef)
[![GitHub License](https://img.shields.io/github/license/ErikKalkoken/everef)](https://github.com/ErikKalkoken/everef?tab=MIT-1-ov-file#readme)

[![download](https://github.com/user-attachments/assets/c8de336f-8c42-4501-86bb-dbc9c66db1f0)](https://github.com/ErikKalkoken/everef/releases/latest)

## Description

**everef** is a command line tool for looking up Eve Online objects. The main audience of this tool are developers of 3rd party apps for EVE Online and admins of tools for Eve Online.

The main benefit of this tool is that is allows you to get the needed information in your current environment without having to switch to another tool, e.g. a browser.

**everef** provides the following features:

- Resolves a list of IDs and/or names to Eve objects.
- Shows details about each object (e.g. the corporation a character belongs to)
- Supports objects of the following categories (same as Universe API endpoints):
  - Agents
  - Alliances
  - Characters
  - Constellations
  - Corporations
  - Factions
  - Regions
  - Stations
  - Solar Systems
  - Types
- Results are shown in pretty tables
- Available for Windows, Linux and macOS

**everef** has to following advantages over using the API directly, e.g. via API explorer:

- Invalid IDs are not omitted, but explicitly mentioned as invalid in the results
- Can resolve a set of IDs even when it contains invalid IDs
- Does not break on temporary outages (e.g. API requests sometimes fail with 503s)
- Fast turnaround due to locally cached results
- Exact name matching (the API sometimes returns objects which similar names, e.g. for "Jita")
- No need to switch from the command line environment to a browser (e.g. by switching to a browser)
- Easier to use then raw curl commands

## Usage

Here is how to use the tool. Just specify a list of IDs and names and the tool will resolve them.

> [!TIP]
> Values are expected to be separated by spaces. To resolve names consisting of multiple words please put them in quotes.

```sh
everef 2119893075 2123140346 "Erik Kalkoken" 603 "Jita" "C C P"
```

Which will print:

```plain
Character:
┌────────────┬─────────────────┬────────────────┬──────────────────┬─────────────┬────────────────────┬───────┐
│     ID     │      NAME       │ CORPORATION ID │ CORPORATION NAME │ ALLIANCE ID │   ALLIANCE NAME    │  NPC  │
├────────────┼─────────────────┼────────────────┼──────────────────┼─────────────┼────────────────────┼───────┤
│ 93330670   │ Erik Kalkoken   │ 98267621       │ The Congregation │ 99013305    │ RAPID HEAVY ROPERS │ false │
├────────────┼─────────────────┼────────────────┼──────────────────┼─────────────┼────────────────────┼───────┤
│ 2119893075 │ CCP Stroopwafel │ 109299958      │ C C P            │ 434243723   │ C C P Alliance     │ false │
├────────────┼─────────────────┼────────────────┼──────────────────┼─────────────┼────────────────────┼───────┤
│ 2123140346 │ CCP Pinky       │ 109299958      │ C C P            │ 434243723   │ C C P Alliance     │ false │
└────────────┴─────────────────┴────────────────┴──────────────────┴─────────────┴────────────────────┴───────┘
Corporation:
┌───────────┬───────┬────────┬─────────┬─────────────┬────────────────┬───────┐
│    ID     │ NAME  │ TICKER │ MEMBERS │ ALLIANCE ID │ ALLIANCE NAME  │  NPC  │
├───────────┼───────┼────────┼─────────┼─────────────┼────────────────┼───────┤
│ 109299958 │ C C P │ -CCP-  │ 963     │ 434243723   │ C C P Alliance │ false │
└───────────┴───────┴────────┴─────────┴─────────────┴────────────────┴───────┘
Inventory Type:
┌─────┬────────┬──────────┬────────────┬─────────────┬───────────────┬───────────┐
│ ID  │  NAME  │ GROUP ID │ GROUP NAME │ CATEGORY ID │ CATEGORY NAME │ PUBLISHED │
├─────┼────────┼──────────┼────────────┼─────────────┼───────────────┼───────────┤
│ 603 │ Merlin │ 25       │ Frigate    │ 6           │ Ship          │ true      │
└─────┴────────┴──────────┴────────────┴─────────────┴───────────────┴───────────┘
Solar System:
┌──────────┬──────┬──────────────────┬────────────────────┬───────────┬─────────────┬────────────┐
│    ID    │ NAME │ CONSTELLATION ID │ CONSTELLATION NAME │ REGION ID │ REGION NAME │  SECURITY  │
├──────────┼──────┼──────────────────┼────────────────────┼───────────┼─────────────┼────────────┤
│ 30000142 │ Jita │ 20000020         │ Kimotoro           │ 10000002  │ The Forge   │ 0.94591314 │
└──────────┴──────┴──────────────────┴────────────────────┴───────────┴─────────────┴────────────┘
```

## Installing

To install everef please download the latest release for your platform from the [releases page](https://github.com/ErikKalkoken/everef/releases). Each release file contains a single executable that can be run directly after decompressing.

> [!NOTE]
> Windows defender (and similar programs) may report everef incorrectly as containing a trojan. This is usually a false positive and caused by a [known issue](https://github.com/microsoft/go/issues/1255) with programs made with the Go programming language. Each release is build from scratch inside a fresh Linux container (not Windows!) provided by Github, so it is highly unlikely to be infected. If this happens to you, please exclude the executable from Windows defender (and similar programs) to proceed.

To test the installation run the following:

```sh
./everef
```

This should print the help page.

> [!TIP]
> To make this command available globally place the executable in a directory that is in your environments's path.
