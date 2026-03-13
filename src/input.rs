use crossterm::event::{self, Event, KeyCode, KeyModifiers};
use crate::editor::Editor;
use crate::types::Mode;
use crate::ui;
use std::io::Result;

pub fn process_keypress(editor: &mut Editor) -> Result<bool> {
    if let Event::Key(key_event) = event::read()? {
        match editor.mode {
            Mode::Normal => handle_normal_mode(editor, key_event),
            Mode::Insert => handle_insert_mode(editor, key_event),
            _ => Ok(false),
        }
    } else {
        Ok(false)
    }
}

fn prompt(editor: &mut Editor, prefix: &str) -> Result<Option<String>> {
    let mut buf = String::new();
    loop {
        editor.set_status(format!("{}{}", prefix, buf));
        ui::refresh_screen(editor)?;
        if let Event::Key(key_event) = event::read()? {
            match key_event.code {
                KeyCode::Char(c) => buf.push(c),
                KeyCode::Backspace => { buf.pop(); }
                KeyCode::Esc => {
                    editor.set_status(String::new());
                    return Ok(None);
                }
                KeyCode::Enter => {
                    editor.set_status(String::new());
                    return Ok(Some(buf));
                }
                _ => {}
            }
        }
    }
}

fn handle_normal_mode(editor: &mut Editor, key: event::KeyEvent) -> Result<bool> {
    if editor.count_prefix > 0 && key.code == KeyCode::Char('0') {
        editor.count_prefix *= 10;
        return Ok(false);
    }
    if let KeyCode::Char(c) = key.code {
        if c.is_ascii_digit() && c != '0' {
            editor.count_prefix = editor.count_prefix * 10 + (c as usize - '0' as usize);
            return Ok(false);
        }
    }

    match key.code {
        KeyCode::Char('i') => {
            editor.mode = Mode::Insert;
            editor.set_status(String::from("-- INSERT --"));
            editor.save_undo();
        }
        KeyCode::Char('a') => {
            if !editor.rows.is_empty() {
                editor.cx = crate::editor::utf8_next_boundary(&editor.rows[editor.cy].s, editor.cx);
            }
            editor.mode = Mode::Insert;
            editor.set_status(String::from("-- INSERT --"));
            editor.save_undo();
        }
        KeyCode::Char('h') | KeyCode::Left => editor.move_cursor(KeyCode::Left),
        KeyCode::Char('j') | KeyCode::Down => editor.move_cursor(KeyCode::Down),
        KeyCode::Char('k') | KeyCode::Up => editor.move_cursor(KeyCode::Up),
        KeyCode::Char('l') | KeyCode::Right => editor.move_cursor(KeyCode::Right),
        KeyCode::Char('w') => editor.move_word_forward(),
        KeyCode::Char('b') => editor.move_word_backward(),
        KeyCode::Char('0') => editor.move_line_start(),
        KeyCode::Char('$') => editor.move_line_end(),
        KeyCode::Char('G') => editor.move_file_end(),
        KeyCode::Char('g') => {
            // Very simple gg implementation
            editor.move_file_start();
        }
        KeyCode::Char('u') => editor.do_undo(),
        KeyCode::Char('r') if key.modifiers.contains(KeyModifiers::CONTROL) => editor.do_redo(),
        KeyCode::Char('/') => {
            if let Some(p) = prompt(editor, "/")? {
                editor.set_search_pattern(p);
                editor.find_next();
            }
        }
        KeyCode::Char('n') => editor.find_next(),
        KeyCode::Char('N') => editor.find_prev(),
        KeyCode::Char(':') => {
            if let Some(cmd) = prompt(editor, ":")? {
                match cmd.as_str() {
                    "q" => return Ok(true),
                    "wq" => {
                        let _ = editor.save_file();
                        return Ok(true);
                    }
                    "w" => { let _ = editor.save_file(); }
                    _ => editor.set_status(format!("Unknown command: {}", cmd)),
                }
            }
        }
        KeyCode::Char('q') if key.modifiers.contains(KeyModifiers::CONTROL) => return Ok(true),
        _ => {}
    }
    editor.count_prefix = 0;
    Ok(false)
}

fn handle_insert_mode(editor: &mut Editor, key: event::KeyEvent) -> Result<bool> {
    match key.code {
        KeyCode::Esc => {
            editor.mode = Mode::Normal;
            if editor.cx > 0 {
                editor.cx = crate::editor::utf8_prev_boundary(&editor.rows[editor.cy].s, editor.cx);
            }
            editor.set_status(String::new());
        }
        KeyCode::Enter => editor.insert_newline(),
        KeyCode::Backspace => editor.del_char(),
        KeyCode::Char(c) => editor.insert_char(c as u8),
        KeyCode::Left | KeyCode::Right | KeyCode::Up | KeyCode::Down => editor.move_cursor(key.code),
        _ => {}
    }
    Ok(false)
}
