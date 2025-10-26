# everef

**everef** is a command line tool for getting information about Eve Online objects.

[![GitHub Release](https://img.shields.io/github/v/release/ErikKalkoken/everef)](https://github.com/ErikKalkoken/everef/everef)
[![build status](https://github.com/ErikKalkoken/everef/actions/workflows/go.yml/badge.svg)](https://github.com/ErikKalkoken/everef/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ErikKalkoken/everef)](https://goreportcard.com/report/github.com/ErikKalkoken/everef)
[![GitHub License](https://img.shields.io/github/license/ErikKalkoken/everef)](https://github.com/ErikKalkoken/everef?tab=MIT-1-ov-file#readme)

[![download](https://github.com/user-attachments/assets/c8de336f-8c42-4501-86bb-dbc9c66db1f0)](https://github.com/ErikKalkoken/everef/releases/latest)

## Description

everef is a command line tool for getting information about EVE Online objects. A typical use case is a developer of 3rd party app for EVE Online, who wants to find out quickly what an EVE ID refers to.

**everef** provides the following features:

- Resolve a list of IDs or names to EVE Entities. Supports the following categories: Agents, Alliances, Characters, Constellations, Corporations, Factions, Regions, Stations, Solar Systems, Types
- Shows unresolvable objects as invalid
- Available for Windows, Linux and macOS

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

## Example usage

Here is how you can resolve IDs:

```sh
everef ids 2119893075 2123140346 603
```

Which will print:

```plain
┌────────────┬─────────────────┬────────────────┐
│     ID     │      NAME       │    CATEGORY    │
├────────────┼─────────────────┼────────────────┤
│ 603        │ Merlin          │ Inventory Type │
├────────────┼─────────────────┼────────────────┤
│ 2119893075 │ CCP Stroopwafel │ Character      │
├────────────┼─────────────────┼────────────────┤
│ 2123140346 │ CCP Pinky       │ Character      │
└────────────┴─────────────────┴────────────────┘
```
