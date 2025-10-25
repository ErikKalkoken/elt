# everef

**everef** is a command line tool for getting information about Eve Online objects.

## Description

**everef** provides the following features:

- Resolve a list of IDs or names to EVE Entities
- Shows unresolvable objects as invalid
- Automatic retries on 50x errors

## Installation

You need to have a Go compiler installed. Then you can install **everef** as follows:

```sh
go install github.com/ErikKalkoken/everef@latest
```

To test that is works run the following

```sh
everef
```

Which should print the help page.

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
