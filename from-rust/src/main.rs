use serde::{Deserialize, Serialize};
use std::{collections::HashMap, fs::read_to_string};
use syn::{Attribute, Expr, ImplItem, Item, Lit, Meta, Type};

#[derive(Default, Debug, Serialize, Deserialize)]
struct MskEnumConst {
    pub name: String,
    pub comment: String,
    pub value: String,
}

#[derive(Default, Debug, Serialize, Deserialize)]
struct MskEnum {
    pub name: String,
    pub comment: String,
    pub enum_consts: Vec<MskEnumConst>,
}

#[derive(Default, Debug, Serialize, Deserialize)]
struct MskFunction {
    pub name: String,
    pub struct_name: String,
    pub comment: String,
}

fn get_comments(s: &syn::Attribute) -> Option<String> {
    if !s.path().is_ident("doc") {
        None
    } else if let Meta::NameValue(nv) = &s.meta {
        if let Expr::Lit(l) = &nv.value {
            if let Lit::Str(sl) = &l.lit {
                Some(sl.value().trim().to_owned())
            } else {
                None
            }
        } else {
            None
        }
    } else {
        None
    }
}

fn get_comment(attrs: &[Attribute]) -> String {
    attrs
        .iter()
        .filter_map(get_comments)
        .collect::<Vec<String>>()
        .join("\n")
}

fn is_enum(name: &str) -> bool {
    name != "Task" && name != "Env" && name != "TaskCB"
}

fn main() {
    let code = read_to_string(std::env::args().nth(1).unwrap()).unwrap();
    let syntax = syn::parse_file(&code).unwrap();

    let mut enums: HashMap<String, MskEnum> = HashMap::new();
    let mut functions: Vec<MskFunction> = vec![];

    for i in syntax.items {
        match i {
            Item::Struct(s) => {
                let name = s.ident.to_string();
                let comment = get_comment(&s.attrs);

                if is_enum(&name) {
                    enums.insert(
                        name.clone(),
                        MskEnum {
                            name: name.clone(),
                            comment: comment.clone(),
                            enum_consts: vec![],
                        },
                    );
                }
            }
            Item::Impl(i) => {
                if i.trait_.is_some() {
                    continue;
                }
                if let Type::Path(p) = i.self_ty.as_ref() {
                    let name = p.path.get_ident().unwrap().to_string();
                    if is_enum(&name) {
                        let v = enums
                            .get_mut(&name)
                            .unwrap_or_else(|| panic!("{name} is no in the map"));
                        for it in i.items {
                            if let ImplItem::Const(c) = it {
                                let c_comment = get_comment(&c.attrs);
                                let c_name = c.ident.to_string();
                                let c_value = if let Expr::Lit(x) = c.expr {
                                    if let Lit::Int(c_n) = x.lit {
                                        c_n.base10_digits().to_owned()
                                    } else {
                                        "".to_owned()
                                    }
                                } else {
                                    "".to_owned()
                                };

                                v.enum_consts.push(MskEnumConst {
                                    name: c_name,
                                    comment: c_comment,
                                    value: c_value,
                                });
                            }
                        }
                    } else {
                        for it in i.items {
                            if let ImplItem::Fn(f) = it {
                                let f_comment = get_comment(&f.attrs);
                                let f_name = f.sig.ident.to_string();
                                functions.push(MskFunction {
                                    name: f_name,
                                    struct_name: name.clone(),
                                    comment: f_comment,
                                });
                            }
                        }
                    }
                }

                // for ii in &i.items {}
            }
            _ => continue,
        };
    }

    std::fs::write("enums.yml", serde_yaml::to_string(&enums).unwrap()).unwrap();
    std::fs::write("funcs.yml", serde_yaml::to_string(&functions).unwrap()).unwrap();
}
