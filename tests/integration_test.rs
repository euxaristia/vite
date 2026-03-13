use videre::Editor;

#[test]
fn test_editor_insertion() {
    let mut editor = Editor::new();
    editor.insert_char('H');
    editor.insert_char('e');
    editor.insert_char('l');
    editor.insert_char('l');
    editor.insert_char('o');
    
    assert_eq!(editor.rows.len(), 1);
    assert_eq!(String::from_utf8_lossy(&editor.rows[0].s), "Hello");
    assert_eq!(editor.cx, 5);
}

#[test]
fn test_editor_newline() {
    let mut editor = Editor::new();
    editor.insert_char('A');
    editor.insert_newline();
    editor.insert_char('B');
    
    assert_eq!(editor.rows.len(), 2);
    assert_eq!(String::from_utf8_lossy(&editor.rows[0].s), "A");
    assert_eq!(String::from_utf8_lossy(&editor.rows[1].s), "B");
    assert_eq!(editor.cy, 1);
    assert_eq!(editor.cx, 1);
}

#[test]
fn test_editor_deletion() {
    let mut editor = Editor::new();
    editor.insert_char('A');
    editor.insert_char('B');
    editor.del_char();
    
    assert_eq!(String::from_utf8_lossy(&editor.rows[0].s), "A");
    assert_eq!(editor.cx, 1);
}

#[test]
fn test_undo_redo() {
    let mut editor = Editor::new();
    editor.insert_char('X');
    assert_eq!(String::from_utf8_lossy(&editor.rows[0].s), "X");
    
    editor.do_undo();
    assert!(editor.rows.is_empty() || editor.rows[0].s.is_empty());
    
    editor.do_redo();
    assert_eq!(String::from_utf8_lossy(&editor.rows[0].s), "X");
}

#[test]
fn test_search() {
    let mut editor = Editor::new();
    editor.insert_row(0, b"find me".to_vec());
    editor.insert_row(1, b"another line".to_vec());
    editor.cx = 0; editor.cy = 0;
    
    editor.set_search_pattern("another".to_string());
    editor.find_next();
    
    assert_eq!(editor.cy, 1);
    assert_eq!(editor.cx, 0);
}

#[test]
fn test_substitute() {
    let mut editor = Editor::new();
    editor.insert_row(0, b"hello world".to_vec());
    editor.cy = 0; editor.cx = 0;
    
    editor.handle_substitute("s/world/rust/");
    assert_eq!(String::from_utf8_lossy(&editor.rows[0].s), "hello rust");
}
