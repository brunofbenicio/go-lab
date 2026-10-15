# Subtests with `t.Run`

## What it solves

A package often has several related behaviors (add, remove, total...) that
deserve separate, named scenarios. `t.Run` turns each scenario into a subtest:
it appears by name in the output, can be selected with `-run`, and a
`t.Fatal` aborts only that subtest instead of the whole function.

## When to use

- To group tests by method or by behavior in a hierarchy (`Parent/Child/Leaf`).
- To share setup between scenarios while keeping each one isolated.
- To run or debug a single scenario without editing the code.

## What this example shows

- Two levels of nesting: `TestCart/Add/merges_quantity_of_the_same_SKU`.
- A fixture function (`newFilledCart`) so each subtest starts from fresh state.
- Subtests created inside a loop; an empty name gets an automatic `#NN` suffix.
- Spaces in names become underscores; match them with `-run`.

## Running

```sh
go test -v ./testing-examples/subtests/
go test -v -run 'TestCart/Add' ./testing-examples/subtests/            # whole group
go test -v -run 'TestCart/Total/with_coupon' ./testing-examples/subtests/
```

In `-run`, each level of the path is its own regular expression, separated by `/`.
