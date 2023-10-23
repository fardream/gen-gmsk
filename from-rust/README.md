# How to run

First, update the data file in `data` folder

```shell
curl -o data/mosek-lib.rs https://raw.githubusercontent.com/MOSEK/mosek.rust/mosek-10.1/src/lib.rs
```

Then, run the code

```shell
cargo run -- ./data/mosek-lib.rs
```
