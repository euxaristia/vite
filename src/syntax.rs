use tree_sitter::{Parser, Node};
use std::path::Path;
use regex::bytes::Regex;
use crate::editor::{Row};
use crate::types::Highlight;

pub fn update_syntax(filename: &str, search_regexp: &Option<Regex>, row_idx: usize, rows: &mut Vec<Row>, force: bool) -> bool {
    if row_idx >= rows.len() { return false; }
    
    if !force && !rows[row_idx].needs_highlight {
        return false;
    }

    let n = rows[row_idx].s.len();
    rows[row_idx].hl = vec![Highlight::Normal; n];
    rows[row_idx].needs_highlight = false;

    let ext = Path::new(filename)
        .extension()
        .and_then(|s| s.to_str())
        .unwrap_or("");

    // Manual C-style comment handling (fallback/extra)
    let mut in_comment = if row_idx > 0 { rows[row_idx - 1].hl_state == 1 } else { false };
    if ext == "c" || ext == "h" || ext == "go" || ext == "rs" {
        let mut i = 0;
        while i < rows[row_idx].s.len() {
            if in_comment {
                rows[row_idx].hl[i] = Highlight::Comment;
                if i + 1 < rows[row_idx].s.len() && rows[row_idx].s[i] == b'*' && rows[row_idx].s[i+1] == b'/' {
                    rows[row_idx].hl[i+1] = Highlight::Comment;
                    i += 2;
                    in_comment = false;
                    continue;
                }
                i += 1;
                continue;
            }
            if i + 1 < rows[row_idx].s.len() && rows[row_idx].s[i] == b'/' && rows[row_idx].s[i+1] == b'*' {
                rows[row_idx].hl[i] = Highlight::Comment;
                rows[row_idx].hl[i+1] = Highlight::Comment;
                i += 2;
                in_comment = true;
                continue;
            }
            i += 1;
        }
    }

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
        "md" => {
            let language = tree_sitter_md::LANGUAGE.into();
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

    let old_state = rows[row_idx].hl_state;
    rows[row_idx].hl_state = if in_comment { 1 } else { 0 };
    rows[row_idx].hl_state != old_state
}

fn apply_tree_sitter_highlight(row: &mut Row, node: &Node) {
    let kind = node.kind();
    let start = node.start_byte();
    let end = node.end_byte();

    let hl = match kind {
        "comment" | "line_comment" | "block_comment" => Highlight::Comment,
        
        "string_literal" | "char_literal" | "string_content" | "string" | "interpreted_string_literal" | "raw_string_literal" => Highlight::String,
        
        "integer_literal" | "number_literal" | "integer" | "float_literal" | "decimal_integer_literal" | "float" => Highlight::Number,
        
        "fn" | "let" | "mut" | "match" | "if" | "else" | "for" | "while" | "loop" | "return" | 
        "struct" | "enum" | "trait" | "impl" | "use" | "pub" | "mod" | "type" | "where" | "async" | "await" |
        "const" | "static" | "extern" | "crate" | "self" | "super" | "move" | "ref" | "dyn" |
        "func" | "package" | "import" | "var" | "go" | "defer" | "chan" | "range" | "map" | "interface" | "select" | "case" | "default" | "switch" | "fallthrough" | "type" |
        "def" | "class" | "try" | "except" | "finally" | "raise" | "with" | "as" | "yield" | "from" | "global" | "nonlocal" | "assert" | "del" | "pass" | "lambda" |
        "if" | "else" | "elif" | "for" | "while" | "break" | "continue" | "return" | "in" | "is" | "not" | "and" | "or" |
        "void" | "int" | "char" | "float" | "double" | "struct" | "enum" | "union" | "typedef" | "extern" | "static" | "const" | "volatile" | "inline" | "restrict" |
        "atx_heading" | "setext_heading" | "heading_content" | "fenced_code_block_delimiter" => Highlight::Keyword1,
        
        "u8" | "u16" | "u32" | "u64" | "u128" | "i8" | "i16" | "i32" | "i64" | "i128" | "f32" | "f64" | 
        "usize" | "isize" | "bool" | "char" | "str" | "String" | "Vec" | "Option" | "Result" | "Box" | "Arc" | "Rc" |
        "int" | "float" | "complex" | "list" | "dict" | "set" | "tuple" | "object" | "None" | "True" | "False" |
        "byte" | "rune" | "uint" | "uintptr" | "float32" | "float64" | "complex64" | "complex128" | "error" |
        "atx_h1_marker" | "atx_h2_marker" | "atx_h3_marker" | "atx_h4_marker" | "atx_h5_marker" | "atx_h6_marker" | "setext_h1_underline" | "setext_h2_underline" |
        "list_marker_plus" | "list_marker_minus" | "list_marker_star" | "list_marker_dot" | "list_marker_parenthesis" |
        "link_text" | "link_label" | "emphasis" | "strong_emphasis" => Highlight::Keyword2,
        
        "fenced_code_block" | "code_fence_content" | "indented_code_block" | "inline_code" | "link_destination" | "uri" => Highlight::String,
        
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
        let changed = update_syntax(filename, search_regexp, i, rows, force);
        if changed && i + 1 < rows.len() {
            rows[i+1].needs_highlight = true;
        }
    }
}
