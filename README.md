# uselocalgomodules

This is a tool to find go modules that you have locally and them as replacements to your `go.mod` if possible.

## Usage

Install:

```
go install github.com/spudtrooper/uselocalgomodules
```

Run in the directory of a `go.mod` file you want to update, e.g.

```
uselocalgomodules
```

By default it will look one directory up, you can increase this with the `--depth` flag.

## Example

* `goutil/go.mod`

    ```
    module github.com/spudtrooper/goutil

    go 1.17

    ...
    ```

* `foo/go.mod`

    ```
    module github.com/spudtrooper/foo

    go 1.17

    ...
    ```

If you run `uselocalgomodules` in the directory `foo`, you have the following added to `go.mod`:

```
replace github.com/spudtrooper/goutil => ../goutil
```

resulting in

```
module github.com/spudtrooper/foo

go 1.17

replace github.com/spudtrooper/goutil => ../goutil

...
```