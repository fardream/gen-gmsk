# How to run

First, update the data file in `data` folder

```shell
curl -o data/mosek-lib.rs https://raw.githubusercontent.com/MOSEK/mosek.rust/refs/heads/mosek-11.2/src/lib.rs
```

Then, run the code

```shell
cargo run -- ./data/mosek-lib.rs
```
