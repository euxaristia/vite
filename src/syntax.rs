use tree_sitter::{Parser, Node};
use std::path::Path;
use regex::bytes::Regex;
use crate::editor::{Row};
use crate::types::Highlight;

pub fn update_syntax(filename: &str, search_regexp: &Option<Regex>, row_idx: usize, rows: &mut Vec<Row>, force: bool) -> bool {
    if row_idx >= rows.len() { return false; }
    
    let n = rows[row_idx].s.len();
    if rows[row_idx].hl.len() != n {
        rows[row_idx].hl = vec![Highlight::Normal; n];
    } else if !force && !rows[row_idx].needs_highlight {
        return false;
    }

    rows[row_idx].needs_highlight = false;
    for i in 0..n {
        rows[row_idx].hl[i] = Highlight::Normal;
    }

    let ext = Path::new(filename)
        .extension()
        .and_then(|s| s.to_str())
        .unwrap_or("");

    let mut parser = Parser::new();
    let lang = match ext {
        "rs" => {
            let language = tree_sitter_rust::LANGUAGE.into();
            parser.set_language(&language).ok();
            Some(language)
        },
        "go" => {
            let language = tree_sitter_go::LANGUAGE.into();
            parser.set_language(&language).ok();
            Some(language)
        },
        "py" => {
            let language = tree_sitter_python::LANGUAGE.into();
            parser.set_language(&language).ok();
            Some(language)
        },
        "c" | "h" => {
            let language = tree_sitter_c::LANGUAGE.into();
            parser.set_language(&language).ok();
            Some(language)
        },
        _ => None,
    };

    if let Some(_) = lang {
        let tree = parser.parse(&rows[row_idx].s, None);
        if let Some(t) = tree {
            let root = t.root_node();
            apply_tree_sitter_highlight(&mut rows[row_idx], &root);
        }
    }

    if let Some(re) = search_regexp {
        let matches: Vec<(usize, usize)> = re.find_iter(&rows[row_idx].s).map(|m| (m.start(), m.end())).collect();
        for (start, end) in matches {
            for i in start..end {
                if i < rows[row_idx].hl.len() {
                    rows[row_idx].hl[i] = Highlight::Match;
                }
            }
        }
    }

    false
}

fn apply_tree_sitter_highlight(row: &mut Row, node: &Node) {
    let kind = node.kind();
    let start = node.start_byte();
    let end = node.end_byte();

    let hl = match kind {
        "comment" | "line_comment" | "block_comment" => Highlight::Comment,
        "string_literal" | "char_literal" | "string_content" | "string" => Highlight::String,
        "integer_literal" | "number_literal" | "integer" | "float_literal" => Highlight::Number,
        "fn" | "let" | "mut" | "match" | "if" | "else" | "for" | "while" | "loop" | "return" | 
        "struct" | "enum" | "trait" | "impl" | "use" | "pub" | "mod" | "type" | "where" | "async" | "await" |
        "const" | "static" | "extern" | "crate" | "self" | "super" | "move" | "ref" | "dyn" |
        "func" | "package" | "import" | "var" | "go" | "defer" | "chan" | "range" | "map" | "interface" |
        "def" | "class" | "try" | "except" | "finally" | "raise" | "with" | "as" | "yield" | "from" | "global" | "nonlocal" | "assert" | "del" | "pass" | "lambda" => Highlight::Keyword1,
        "u8" | "u16" | "u32" | "u64" | "u128" | "i8" | "i16" | "i32" | "i64" | "i128" | "f32" | "f64" | 
        "usize" | "isize" | "bool" | "char" | "str" | "String" | "Vec" | "Option" | "Result" |
        "int" | "float" | "complex" | "list" | "dict" | "set" | "tuple" | "object" | "None" | "True" | "False" |
        "byte" | "rune" | "uint" | "uintptr" | "float32" | "float64" | "complex64" | "complex128" | "error" => Highlight::Keyword2,
        _ => Highlight::Normal,
    };

    if hl != Highlight::Normal {
        for i in start..end {
            if i < row.hl.len() {
                row.hl[i] = hl;
            }
        }
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i as u32) {
            apply_tree_sitter_highlight(row, &child);
        }
    }
}

pub fn update_all_syntax(filename: &str, search_regexp: &Option<Regex>, rows: &mut Vec<Row>, force: bool) {
    for i in 0..rows.len() {
        update_syntax(filename, search_regexp, i, rows, force);
    }
}
