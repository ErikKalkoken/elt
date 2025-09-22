# eveid

**eveid** is a command line tool for resolving Eve Online IDs to names and categories.

## Description

**eveid** provides the following features:

- Resolves a list of IDs to their categories and names directly from the command line
- Output is JSON
- Can resolve a mix of valid and invalid IDs
- Automatic retries on 50x errors

## Installation

You need to have a Go compiler installed. Then you can install **eveid** with:

```sh
go install github.com/ErikKalkoken/eveid@latest
```

## Example usage

Here is how you can resolve IDs:

```sh
eveid 2119893075,2123140346
```

Which will return:

```json
[
    {
        "id": 2119893075,
        "category": "character",
        "name": "CCP Stroopwafel"
    },
    {
        "id": 2123140346,
        "category": "character",
        "name": "CCP Pinky"
    }
]
```
