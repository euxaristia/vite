use crossterm::event::{self, Event, KeyCode, KeyModifiers};
use crate::editor::Editor;
use crate::types::Mode;
use crate::ui;
use std::io::Result;

pub fn process_keypress(editor: &mut Editor) -> Result<bool> {
    match event::read()? {
        Event::Key(key_event) => {
            match editor.mode {
                Mode::Normal => handle_normal_mode(editor, key_event),
                Mode::Insert => handle_insert_mode(editor, key_event),
                Mode::Visual | Mode::VisualLine => handle_visual_mode(editor, key_event),
            }
        }
        Event::Resize(cols, rows) => {
            editor.screen_cols = cols as usize;
            editor.screen_rows = rows.saturating_sub(2) as usize;
            Ok(false)
        }
        _ => Ok(false),
    }
}

fn prompt<F>(editor: &mut Editor, prefix: &str, mut callback: F) -> Result<Option<String>> 
where F: FnMut(&mut Editor, &str, KeyCode) {
    let mut buf = String::new();
    loop {
        editor.set_status(format!("{}{}", prefix, buf));
        ui::refresh_screen(editor)?;
        match event::read()? {
            Event::Key(key_event) => {
                match key_event.code {
                    KeyCode::Char(c) => {
                        buf.push(c);
                        callback(editor, &buf, key_event.code);
                    }
                    KeyCode::Backspace => {
                        if buf.is_empty() {
                            editor.set_status(String::new());
                            callback(editor, &buf, KeyCode::Backspace);
                            return Ok(None);
                        }
                        buf.pop();
                        callback(editor, &buf, key_event.code);
                    }
                    KeyCode::Esc => {
                        editor.set_status(String::new());
                        callback(editor, &buf, KeyCode::Esc);
                        return Ok(None);
                    }
                    KeyCode::Enter => {
                        editor.set_status(String::new());
                        callback(editor, &buf, KeyCode::Enter);
                        return Ok(Some(buf));
                    }
                    _ => {
                        callback(editor, &buf, key_event.code);
                    }
                }
            }
            Event::Resize(cols, rows) => {
                editor.screen_cols = cols as usize;
                editor.screen_rows = rows.saturating_sub(2) as usize;
            }
            _ => {}
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
            if editor.mode == Mode::Visual {
                editor.mode = Mode::Normal;
                editor.set_status(String::new());
            } else {
                editor.mode = Mode::Visual;
                editor.sel_sx = editor.cx;
                editor.sel_sy = editor.cy;
            }
        }
        KeyCode::Char('V') => {
            if editor.mode == Mode::VisualLine {
                editor.mode = Mode::Normal;
                editor.set_status(String::new());
            } else {
                editor.mode = Mode::VisualLine;
                editor.sel_sx = 0;
                editor.sel_sy = editor.cy;
            }
        }
        KeyCode::Char('h') | KeyCode::Left => { for _ in 0..count { editor.move_cursor(KeyCode::Left); } }
        KeyCode::Char('j') | KeyCode::Down => { for _ in 0..count { editor.move_cursor(KeyCode::Down); } }
        KeyCode::Char('k') | KeyCode::Up => { for _ in 0..count { editor.move_cursor(KeyCode::Up); } }
        KeyCode::Char('l') | KeyCode::Right => { for _ in 0..count { editor.move_cursor(KeyCode::Right); } }
        KeyCode::Char('w') => { for _ in 0..count { editor.move_word_forward(false); } }
        KeyCode::Char('W') => { for _ in 0..count { editor.move_word_forward(true); } }
        KeyCode::Char('b') => { for _ in 0..count { editor.move_word_backward(false); } }
        KeyCode::Char('B') => { for _ in 0..count { editor.move_word_backward(true); } }
        KeyCode::Char('e') => { for _ in 0..count { editor.move_word_end(false); } }
        KeyCode::Char('E') => { for _ in 0..count { editor.move_word_end(true); } }
        KeyCode::Char('{') => { for _ in 0..count { editor.move_prev_paragraph(); } }
        KeyCode::Char('}') => { for _ in 0..count { editor.move_next_paragraph(); } }
        KeyCode::Char('%') => editor.match_bracket(),
        KeyCode::Char('0') => editor.move_line_start(),
        KeyCode::Char('^') => editor.move_first_non_whitespace(),
        KeyCode::Char('$') => editor.move_line_end(),
        KeyCode::Char('G') => {
            if editor.count_prefix > 0 {
                let target = editor.count_prefix.saturating_sub(1);
                editor.cy = target.min(editor.rows.len().saturating_sub(1));
                editor.move_line_start();
            } else {
                editor.move_file_end();
            }
        }
        KeyCode::Char('g') => {
            if let Event::Key(next_key) = event::read()? {
                if next_key.code == KeyCode::Char('g') {
                    if editor.count_prefix > 0 {
                        let target = editor.count_prefix.saturating_sub(1);
                        editor.cy = target.min(editor.rows.len().saturating_sub(1));
                        editor.move_line_start();
                    } else {
                        editor.move_file_start();
                    }
                }
            }
        }
        KeyCode::Char('m') => {
            if let Event::Key(next_key) = event::read()? {
                if let KeyCode::Char(m) = next_key.code {
                    if m >= 'a' && m <= 'z' {
                        let i = (m as u8 - b'a') as usize;
                        editor.mark_set[i] = true;
                        editor.marks_x[i] = editor.cx;
                        editor.marks_y[i] = editor.cy;
                    }
                }
            }
        }
        KeyCode::Char('\'') => {
            if let Event::Key(next_key) = event::read()? {
                if let KeyCode::Char(m) = next_key.code {
                    if m >= 'a' && m <= 'z' {
                        let i = (m as u8 - b'a') as usize;
                        if editor.mark_set[i] {
                            editor.cy = editor.marks_y[i].min(editor.rows.len().saturating_sub(1));
                            editor.cx = editor.marks_x[i].min(editor.rows[editor.cy].s.len());
                            editor.preferred = editor.cx;
                        } else {
                            editor.set_status("Mark not set".into());
                        }
                    }
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
        KeyCode::Char('y') | KeyCode::Char('d') | KeyCode::Char('c') | KeyCode::Char('>') | KeyCode::Char('<') => {
            if let KeyCode::Char(op) = key.code {
                handle_operator(editor, op, count)?;
            }
        }
        KeyCode::Char('p') => { for _ in 0..count { editor.paste(); } }
        KeyCode::Char('/') => {
            let saved_x = editor.cx; let saved_y = editor.cy;
            let saved_pref = editor.preferred; let saved_col = editor.coloff; let saved_row = editor.rowoff;
            
            let res = prompt(editor, "/", |ed, query, key| {
                if key == KeyCode::Esc || key == KeyCode::Backspace && query.is_empty() {
                    ed.set_search_pattern(String::new());
                } else if key == KeyCode::Enter {
                    // keep current position
                } else {
                    ed.set_search_pattern(query.to_string());
                    ed.find_next();
                }
            })?;
            
            if res.is_none() {
                editor.cx = saved_x; editor.cy = saved_y;
                editor.preferred = saved_pref; editor.coloff = saved_col; editor.rowoff = saved_row;
            }
        }
        KeyCode::Char('n') => { for _ in 0..count { editor.find_next(); } }
        KeyCode::Char('N') => { for _ in 0..count { editor.find_prev(); } }
        KeyCode::Char(';') => { for _ in 0..count { editor.repeat_char_search(false); } }
        KeyCode::Char(',') => { for _ in 0..count { editor.repeat_char_search(true); } }
        KeyCode::Char(':') => {
            if let Some(cmd) = prompt(editor, ":", |_, _, _| {})? {
                let parts: Vec<&str> = cmd.split_whitespace().collect();
                if parts.is_empty() { return Ok(false); }
                match parts[0] {
                    "q" => {
                        if editor.dirty { editor.set_status("No write since last change (add ! to override)".into()); }
                        else { return Ok(true); }
                    }
                    "q!" | "qa!" => return Ok(true),
                    "qa" => {
                        if editor.dirty { editor.set_status("No write since last change (add ! to override)".into()); }
                        else { return Ok(true); }
                    }
                    "w" => { let _ = editor.save_file(); }
                    "wq" | "x" => {
                        let _ = editor.save_file();
                        return Ok(true);
                    }
                    "e" => {
                        if parts.len() > 1 {
                            if editor.dirty {
                                editor.set_status("No write since last change (add ! to override)".into());
                            } else {
                                let _ = editor.open_file(parts[1]);
                                editor.move_file_start();
                            }
                        }
                    }
                    "h" | "help" => {
                        editor.set_status("Help not implemented yet".into());
                    }
                    _ if parts[0].starts_with('s') || parts[0].starts_with("%s") => {
                        editor.handle_substitute(&cmd);
                    }
                    _ => {
                        if let Ok(n) = parts[0].parse::<usize>() {
                            let target = n.saturating_sub(1);
                            editor.cy = target.min(editor.rows.len().saturating_sub(1));
                            editor.move_line_start();
                        } else {
                            editor.set_status(format!("Not an editor command: {}", parts[0]));
                        }
                    }
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
        KeyCode::Char('f') | KeyCode::Char('F') | KeyCode::Char('t') | KeyCode::Char('T') => {
            let mode_char = match key.code {
                KeyCode::Char(c) => c,
                _ => 'f',
            };
            if let Event::Key(next_key) = event::read()? {
                if let KeyCode::Char(n) = next_key.code {
                    let dir = if mode_char == 'F' || mode_char == 'T' { -1 } else { 1 };
                    let till = mode_char == 't' || mode_char == 'T';
                    for _ in 0..count {
                        if !editor.find_char(n as u8, dir, till) { break; }
                    }
                }
            }
        }
        KeyCode::Char('Z') => {
            if let Event::Key(next_key) = event::read()? {
                match next_key.code {
                    KeyCode::Char('Z') => {
                        let _ = editor.save_file();
                        return Ok(true);
                    }
                    KeyCode::Char('Q') => {
                        return Ok(true);
                    }
                    _ => {}
                }
            }
        }
        _ => {}
    }
    editor.count_prefix = 0;
    Ok(false)
}

fn operator_exclusive_motion(m: char) -> bool {
    match m {
        'w' | 'W' | 'b' | 'B' | '0' | '^' | '{' | '}' | 'h' | 'j' | 'k' | 'l' | 'g' | 'G' => true,
        _ => false,
    }
}

fn handle_operator(editor: &mut Editor, op: char, count: usize) -> Result<()> {
    let start_x = editor.cx;
    let start_y = editor.cy;
    
    if let Event::Key(m_key) = event::read()? {
        if let KeyCode::Char(m) = m_key.code {
            if m == op {
                let sy = editor.cy;
                let ey = (editor.cy + count).saturating_sub(1).min(editor.rows.len().saturating_sub(1));
                
                if op == '>' || op == '<' {
                    editor.save_undo();
                    for y in sy..=ey {
                        editor.rows[y] = editor.rows[y].duplicate();
                        if op == '>' {
                            editor.rows[y].s.insert(0, b' '); editor.rows[y].s.insert(0, b' ');
                            editor.rows[y].s.insert(0, b' '); editor.rows[y].s.insert(0, b' ');
                        } else {
                            let mut trim = 0;
                            while trim < 4 && trim < editor.rows[y].s.len() && editor.rows[y].s[trim] == b' ' {
                                trim += 1;
                            }
                            if trim > 0 { editor.rows[y].s.drain(0..trim); }
                        }
                        editor.rows[y].needs_highlight = true;
                    }
                    editor.dirty = true;
                    return Ok(());
                }

                editor.yoink(0, sy, 0, ey, true);
                if op != 'y' {
                    for _ in 0..=(ey - sy) {
                        editor.del_row(sy);
                    }
                    if editor.rows.is_empty() {
                        editor.insert_row(0, Vec::new());
                    }
                    editor.cy = sy.min(editor.rows.len().saturating_sub(1));
                    editor.cx = 0;
                    editor.preferred = 0;
                    if op == 'c' {
                        editor.mode = Mode::Insert;
                        editor.set_status("-- INSERT --".into());
                    }
                }
                editor.selected_register = '"' as usize;
                return Ok(());
            }
            
            if m == 'i' || m == 'a' {
                if let Event::Key(obj_key) = event::read()? {
                    if let KeyCode::Char(obj) = obj_key.code {
                        if let Some((sx, sy, ex, ey)) = editor.find_text_object(obj, m == 'i') {
                            editor.yoink(sx, sy, ex, ey, false);
                            if op != 'y' {
                                editor.delete_range(sx, sy, ex, ey);
                                if op == 'c' {
                                    editor.mode = Mode::Insert;
                                    editor.set_status("-- INSERT --".into());
                                }
                            } else {
                                editor.cx = start_x; editor.cy = start_y;
                            }
                            editor.selected_register = '"' as usize;
                            return Ok(());
                        }
                    }
                }
                return Ok(());
            }

            // Handle motions
            match m {
                'w' => { for _ in 0..count { editor.move_word_forward(false); } }
                'W' => { for _ in 0..count { editor.move_word_forward(true); } }
                'b' => { for _ in 0..count { editor.move_word_backward(false); } }
                'B' => { for _ in 0..count { editor.move_word_backward(true); } }
                'e' => { for _ in 0..count { editor.move_word_end(false); } }
                'E' => { for _ in 0..count { editor.move_word_end(true); } }
                'h' => { for _ in 0..count { editor.move_cursor(KeyCode::Left); } }
                'j' => { for _ in 0..count { editor.move_cursor(KeyCode::Down); } }
                'k' => { for _ in 0..count { editor.move_cursor(KeyCode::Up); } }
                'l' => { for _ in 0..count { editor.move_cursor(KeyCode::Right); } }
                '{' => { for _ in 0..count { editor.move_prev_paragraph(); } }
                '}' => { for _ in 0..count { editor.move_next_paragraph(); } }
                '$' => editor.move_line_end(),
                '0' => editor.move_line_start(),
                '^' => editor.move_first_non_whitespace(),
                _ => { return Ok(()); }
            }
            
            let dest_x = editor.cx;
            let dest_y = editor.cy;
            
            if dest_x == start_x && dest_y == start_y { return Ok(()); }
            
            let (sx, sy, ex, ey);
            if editor.pos_before(start_x, start_y, dest_x, dest_y) {
                sx = start_x; sy = start_y;
                if operator_exclusive_motion(m) {
                    if let Some((px, py)) = editor.prev_pos(dest_x, dest_y) {
                        ex = px; ey = py;
                    } else {
                        ex = dest_x; ey = dest_y;
                    }
                } else {
                    ex = dest_x; ey = dest_y;
                }
            } else {
                sx = dest_x; sy = dest_y;
                if operator_exclusive_motion(m) {
                    if let Some((px, py)) = editor.prev_pos(start_x, start_y) {
                        ex = px; ey = py;
                    } else {
                        ex = start_x; ey = start_y;
                    }
                } else {
                    ex = start_x; ey = start_y;
                }
            }

            if op == '>' || op == '<' {
                editor.save_undo();
                for y in sy..=ey {
                    editor.rows[y] = editor.rows[y].duplicate();
                    if op == '>' {
                        editor.rows[y].s.insert(0, b' '); editor.rows[y].s.insert(0, b' ');
                        editor.rows[y].s.insert(0, b' '); editor.rows[y].s.insert(0, b' ');
                    } else {
                        let mut trim = 0;
                        while trim < 4 && trim < editor.rows[y].s.len() && editor.rows[y].s[trim] == b' ' {
                            trim += 1;
                        }
                        if trim > 0 { editor.rows[y].s.drain(0..trim); }
                    }
                    editor.rows[y].needs_highlight = true;
                }
                editor.dirty = true;
                editor.cx = start_x; editor.cy = start_y;
                return Ok(());
            }

            editor.yoink(sx, sy, ex, ey, false);
            if op != 'y' {
                editor.delete_range(sx, sy, ex, ey);
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
        KeyCode::Char('i') | KeyCode::Char('a') => {
            if let Event::Key(obj_key) = event::read()? {
                if let KeyCode::Char(obj) = obj_key.code {
                    if let Some((sx, sy, ex, ey)) = editor.find_text_object(obj, key.code == KeyCode::Char('i')) {
                        editor.sel_sx = sx; editor.sel_sy = sy;
                        editor.cx = ex; editor.cy = ey;
                        editor.preferred = ex;
                        editor.mode = Mode::Visual;
                    }
                }
            }
        }
        KeyCode::Char('h') | KeyCode::Left => editor.move_cursor(KeyCode::Left),
        KeyCode::Char('j') | KeyCode::Down => editor.move_cursor(KeyCode::Down),
        KeyCode::Char('k') | KeyCode::Up => editor.move_cursor(KeyCode::Up),
        KeyCode::Char('l') | KeyCode::Right => editor.move_cursor(KeyCode::Right),
        KeyCode::Char('w') => editor.move_word_forward(false),
        KeyCode::Char('W') => editor.move_word_forward(true),
        KeyCode::Char('b') => editor.move_word_backward(false),
        KeyCode::Char('B') => editor.move_word_backward(true),
        KeyCode::Char('e') => editor.move_word_end(false),
        KeyCode::Char('E') => editor.move_word_end(true),
        KeyCode::Char('{') => editor.move_prev_paragraph(),
        KeyCode::Char('}') => editor.move_next_paragraph(),
        KeyCode::Char('0') => editor.move_line_start(),
        KeyCode::Char('^') => editor.move_first_non_whitespace(),
        KeyCode::Char('$') => editor.move_line_end(),
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
        KeyCode::Char('>') | KeyCode::Char('<') => {
            let (mut sy, mut ey) = (editor.sel_sy, editor.cy);
            if sy > ey { std::mem::swap(&mut sy, &mut ey); }
            editor.save_undo();
            for y in sy..=ey {
                if y < editor.rows.len() {
                    editor.rows[y] = editor.rows[y].duplicate();
                    if key.code == KeyCode::Char('>') {
                        editor.rows[y].s.insert(0, b' '); editor.rows[y].s.insert(0, b' ');
                        editor.rows[y].s.insert(0, b' '); editor.rows[y].s.insert(0, b' ');
                    } else {
                        let mut trim = 0;
                        while trim < 4 && trim < editor.rows[y].s.len() && editor.rows[y].s[trim] == b' ' {
                            trim += 1;
                        }
                        if trim > 0 { editor.rows[y].s.drain(0..trim); }
                    }
                    editor.rows[y].needs_highlight = true;
                }
            }
            editor.dirty = true;
            editor.mode = Mode::Normal;
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
        KeyCode::Tab => {
            for _ in 0..4 { editor.insert_char(' '); }
        }
        KeyCode::Backspace => editor.del_char(),
        KeyCode::Char(c) => editor.insert_char(c),
        KeyCode::Left | KeyCode::Right | KeyCode::Up | KeyCode::Down => editor.move_cursor(key.code),
        _ => {}
    }
    Ok(false)
}
