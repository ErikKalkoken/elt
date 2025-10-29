# elt

**elt** is a command line tool for looking up Eve Online objects.

[![GitHub Release](https://img.shields.io/github/v/release/ErikKalkoken/elt)](https://github.com/ErikKalkoken/elt/elt)
[![build status](https://github.com/ErikKalkoken/elt/actions/workflows/go.yml/badge.svg)](https://github.com/ErikKalkoken/elt/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ErikKalkoken/elt)](https://goreportcard.com/report/github.com/ErikKalkoken/elt)
[![GitHub License](https://img.shields.io/github/license/ErikKalkoken/elt)](https://github.com/ErikKalkoken/elt?tab=MIT-1-ov-file#readme)

[![download](https://github.com/user-attachments/assets/c8de336f-8c42-4501-86bb-dbc9c66db1f0)](https://github.com/ErikKalkoken/elt/releases/latest)

## Description

**elt** is a command line tool that looks up EVE Online objects from the game server and prints them in the terminal. It provides a convenient and fast alternative to using a browser or curl commands for quickly resolving Eve IDs or names.

Main features:

- Takes Eve IDs and names as input
- Shows all found Eve object with details in a pretty table
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
- Available for Windows, Linux and macOS

**elt** is an short for EVE Lookup Tool.

## Usage

**elt** is very easy to use. Just give it a list of IDs and/or names and the tool will resolve them. If one of the inputs is invalid or can not be resolved the tool continue to resolve the other inputs and tell you about the invalids ones in the result table.

```sh
elt 2119893075 2123140346 "Erik Kalkoken" 603 "Jita" "C C P"
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

To install **elt** please download the latest release for your platform from the [releases page](https://github.com/ErikKalkoken/elt/releases). Each release file contains a single executable that can be run directly after decompressing.

> [!NOTE]
> Windows defender (and similar programs) may report **elt** incorrectly as containing a trojan. This is usually a false positive and caused by a [known issue](https://github.com/microsoft/go/issues/1255) with programs made with the Go programming language. Each release is build from scratch inside a fresh Linux container (not Windows!) provided by Github, so it is highly unlikely to be infected. If this happens to you, please exclude the executable from Windows defender (and similar programs) to proceed.

To test the installation run the following:

```sh
./elt
```

This should print the help page.

> [!TIP]
> To make this command available globally place the executable in a directory that is in your environments's path.
