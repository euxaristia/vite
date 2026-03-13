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
            Mode::Visual | Mode::VisualLine => handle_visual_mode(editor, key_event),
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
        if c.is_ascii_digit() && c != '0' && !key.modifiers.contains(KeyModifiers::CONTROL) {
            editor.count_prefix = editor.count_prefix * 10 + (c as usize - '0' as usize);
            return Ok(false);
        }
    }

    let count = if editor.count_prefix == 0 { 1 } else { editor.count_prefix };

    match key.code {
        KeyCode::Char('c') if key.modifiers.contains(KeyModifiers::CONTROL) => {
            if editor.dirty && editor.quit_warn_remaining > 0 {
                editor.set_status(format!("WARNING!!! Unsaved changes. Press Ctrl-C {} more times to quit.", editor.quit_warn_remaining));
                editor.quit_warn_remaining -= 1;
            } else {
                return Ok(true);
            }
        }
        KeyCode::Char('a') if key.modifiers.contains(KeyModifiers::CONTROL) => editor.increment_number(count as i32),
        KeyCode::Char('x') if key.modifiers.contains(KeyModifiers::CONTROL) => editor.increment_number(-(count as i32)),
        KeyCode::Char('r') if key.modifiers.contains(KeyModifiers::CONTROL) => { for _ in 0..count { editor.do_redo(); } }
        
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
        KeyCode::Char('I') => {
            editor.move_first_non_whitespace();
            editor.mode = Mode::Insert;
            editor.set_status(String::from("-- INSERT --"));
            editor.save_undo();
        }
        KeyCode::Char('A') => {
            editor.move_line_end();
            if !editor.rows.is_empty() && editor.cx < editor.rows[editor.cy].s.len() {
                editor.cx = crate::editor::utf8_next_boundary(&editor.rows[editor.cy].s, editor.cx);
            }
            editor.mode = Mode::Insert;
            editor.set_status(String::from("-- INSERT --"));
            editor.save_undo();
        }
        KeyCode::Char('o') => {
            if editor.rows.is_empty() {
                editor.insert_row(0, Vec::new());
                editor.cy = 0; editor.cx = 0;
            } else {
                editor.move_line_end();
                if editor.cx < editor.rows[editor.cy].s.len() {
                    editor.cx = crate::editor::utf8_next_boundary(&editor.rows[editor.cy].s, editor.cx);
                }
            }
            editor.insert_newline();
            editor.mode = Mode::Insert;
            editor.set_status(String::from("-- INSERT --"));
            editor.save_undo();
        }
        KeyCode::Char('O') => {
            if editor.rows.is_empty() {
                editor.insert_row(0, Vec::new());
                editor.cy = 0; editor.cx = 0;
            } else {
                editor.cx = 0;
                editor.insert_newline();
                editor.cy = editor.cy.saturating_sub(1);
                editor.cx = 0; editor.preferred = 0;
            }
            editor.mode = Mode::Insert;
            editor.set_status(String::from("-- INSERT --"));
            editor.save_undo();
        }
        KeyCode::Char('v') => {
            editor.mode = Mode::Visual;
            editor.sel_sx = editor.cx;
            editor.sel_sy = editor.cy;
        }
        KeyCode::Char('V') => {
            editor.mode = Mode::VisualLine;
            editor.sel_sx = 0;
            editor.sel_sy = editor.cy;
        }
        KeyCode::Char('h') | KeyCode::Left => { for _ in 0..count { editor.move_cursor(KeyCode::Left); } }
        KeyCode::Char('j') | KeyCode::Down => { for _ in 0..count { editor.move_cursor(KeyCode::Down); } }
        KeyCode::Char('k') | KeyCode::Up => { for _ in 0..count { editor.move_cursor(KeyCode::Up); } }
        KeyCode::Char('l') | KeyCode::Right => { for _ in 0..count { editor.move_cursor(KeyCode::Right); } }
        KeyCode::Char('w') => { for _ in 0..count { editor.move_word_forward(); } }
        KeyCode::Char('b') => { for _ in 0..count { editor.move_word_backward(); } }
        KeyCode::Char('0') => editor.move_line_start(),
        KeyCode::Char('$') => editor.move_line_end(),
        KeyCode::Char('G') => editor.move_file_end(),
        KeyCode::Char('g') => {
            if let Event::Key(next_key) = event::read()? {
                if next_key.code == KeyCode::Char('g') {
                    editor.move_file_start();
                }
            }
        }
        KeyCode::Char('u') => { for _ in 0..count { editor.do_undo(); } }
        KeyCode::Char('x') => {
            let sx = editor.cx;
            let sy = editor.cy;
            let ex = (editor.cx + count).saturating_sub(1);
            editor.yoink(sx, sy, ex, sy, false);
            editor.delete_range(sx, sy, ex, sy);
        }
        KeyCode::Char('y') | KeyCode::Char('d') | KeyCode::Char('c') => {
            if let KeyCode::Char(op) = key.code {
                handle_operator(editor, op, count)?;
            }
        }
        KeyCode::Char('p') => { for _ in 0..count { editor.paste(); } }
        KeyCode::Char('/') => {
            if let Some(p) = prompt(editor, "/")? {
                editor.set_search_pattern(p);
                editor.find_next();
            }
        }
        KeyCode::Char('n') => { for _ in 0..count { editor.find_next(); } }
        KeyCode::Char('N') => { for _ in 0..count { editor.find_prev(); } }
        KeyCode::Char(':') => {
            if let Some(cmd) = prompt(editor, ":")? {
                let parts: Vec<&str> = cmd.split_whitespace().collect();
                if parts.is_empty() { return Ok(false); }
                match parts[0] {
                    "q" => {
                        if editor.dirty { editor.set_status("No write since last change".into()); }
                        else { return Ok(true); }
                    }
                    "q!" => return Ok(true),
                    "w" => { let _ = editor.save_file(); }
                    "wq" => {
                        let _ = editor.save_file();
                        return Ok(true);
                    }
                    _ => editor.set_status(format!("Unknown command: {}", parts[0])),
                }
            }
        }
        KeyCode::Char('"') => {
            editor.set_status("\"".into());
            ui::refresh_screen(editor)?;
            if let Event::Key(reg_key) = event::read()? {
                if let KeyCode::Char(r) = reg_key.code {
                    editor.selected_register = r as usize;
                    editor.set_status(format!("\"{}", r));
                    ui::refresh_screen(editor)?;
                }
            }
            return Ok(false);
        }
        _ => {}
    }
    editor.count_prefix = 0;
    Ok(false)
}

fn handle_operator(editor: &mut Editor, op: char, count: usize) -> Result<()> {
    let start_x = editor.cx;
    let start_y = editor.cy;
    
    if let Event::Key(m_key) = event::read()? {
        if let KeyCode::Char(m) = m_key.code {
            if m == op {
                let sy = editor.cy;
                let ey = (editor.cy + count).saturating_sub(1).min(editor.rows.len().saturating_sub(1));
                editor.yoink(0, sy, 0, ey, true);
                if op != 'y' {
                    editor.delete_range(0, sy, 0, ey);
                    if op == 'c' {
                        editor.mode = Mode::Insert;
                        editor.set_status("-- INSERT --".into());
                    }
                }
                editor.selected_register = '"' as usize;
                return Ok(());
            }
            
            // Handle basic motions for operators
            match m {
                'w' => { for _ in 0..count { editor.move_word_forward(); } }
                'b' => { for _ in 0..count { editor.move_word_backward(); } }
                'h' => { for _ in 0..count { editor.move_cursor(KeyCode::Left); } }
                'j' => { for _ in 0..count { editor.move_cursor(KeyCode::Down); } }
                'k' => { for _ in 0..count { editor.move_cursor(KeyCode::Up); } }
                'l' => { for _ in 0..count { editor.move_cursor(KeyCode::Right); } }
                '$' => editor.move_line_end(),
                '0' => editor.move_line_start(),
                _ => { return Ok(()); }
            }
            
            let dest_x = editor.cx;
            let dest_y = editor.cy;
            
            if dest_x == start_x && dest_y == start_y { return Ok(()); }
            
            editor.yoink(start_x, start_y, dest_x, dest_y, false);
            if op != 'y' {
                editor.delete_range(start_x, start_y, dest_x, dest_y);
                if op == 'c' {
                    editor.mode = Mode::Insert;
                    editor.set_status("-- INSERT --".into());
                }
            } else {
                editor.cx = start_x;
                editor.cy = start_y;
                editor.preferred = start_x;
            }
            editor.selected_register = '"' as usize;
        }
    }
    Ok(())
}

fn handle_visual_mode(editor: &mut Editor, key: event::KeyEvent) -> Result<bool> {
    match key.code {
        KeyCode::Esc => {
            editor.mode = Mode::Normal;
            editor.set_status(String::new());
        }
        KeyCode::Char('h') | KeyCode::Left => editor.move_cursor(KeyCode::Left),
        KeyCode::Char('j') | KeyCode::Down => editor.move_cursor(KeyCode::Down),
        KeyCode::Char('k') | KeyCode::Up => editor.move_cursor(KeyCode::Up),
        KeyCode::Char('l') | KeyCode::Right => editor.move_cursor(KeyCode::Right),
        KeyCode::Char('y') => {
            editor.yoink(editor.sel_sx, editor.sel_sy, editor.cx, editor.cy, editor.mode == Mode::VisualLine);
            editor.mode = Mode::Normal;
        }
        KeyCode::Char('d') | KeyCode::Char('x') => {
            editor.yoink(editor.sel_sx, editor.sel_sy, editor.cx, editor.cy, editor.mode == Mode::VisualLine);
            editor.delete_range(editor.sel_sx, editor.sel_sy, editor.cx, editor.cy);
            editor.mode = Mode::Normal;
        }
        KeyCode::Char('c') => {
            editor.yoink(editor.sel_sx, editor.sel_sy, editor.cx, editor.cy, editor.mode == Mode::VisualLine);
            editor.delete_range(editor.sel_sx, editor.sel_sy, editor.cx, editor.cy);
            editor.mode = Mode::Insert;
            editor.set_status(String::from("-- INSERT --"));
        }
        _ => {}
    }
    Ok(false)
}

fn handle_insert_mode(editor: &mut Editor, key: event::KeyEvent) -> Result<bool> {
    match key.code {
        KeyCode::Esc => {
            editor.mode = Mode::Normal;
            if !editor.rows.is_empty() && editor.cx > 0 {
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
