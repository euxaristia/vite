use std::fs::File;
use std::io::{self, BufRead, BufReader, Write};
use std::path::Path;
use std::time::Instant;
use regex::bytes::Regex;
use arboard::Clipboard;
use std::process::Command;

use crate::types::{Mode, Highlight, Reg};

#[derive(Clone, Debug)]
pub struct Row {
    pub idx: usize,
    pub s: Vec<u8>,
    pub hl: Vec<Highlight>,
    pub open: bool,
    pub needs_highlight: bool,
    pub hl_state: i32,
}

impl Row {
    pub fn new(idx: usize, s: Vec<u8>) -> Self {
        Self {
            idx,
            s,
            hl: Vec::new(),
            open: false,
            needs_highlight: true,
            hl_state: 0,
        }
    }

    pub fn duplicate(&self) -> Self {
        Self {
            idx: self.idx,
            s: self.s.clone(),
            hl: self.hl.clone(),
            open: self.open,
            needs_highlight: self.needs_highlight,
            hl_state: self.hl_state,
        }
    }
}

#[derive(Clone, Debug)]
pub struct UndoState {
    pub rows: Vec<Row>,
    pub cx: usize,
    pub cy: usize,
}

pub struct Editor {
    pub cx: usize,
    pub cy: usize,
    pub preferred: usize,
    pub rowoff: usize,
    pub coloff: usize,
    pub screen_rows: usize,
    pub screen_cols: usize,
    pub rows: Vec<Row>,
    pub dirty: bool,
    pub filename: String,
    pub git_status: String,
    pub statusmsg: String,
    pub status_time: Option<Instant>,
    pub mode: Mode,
    pub sel_sx: usize,
    pub sel_sy: usize,
    pub search_pattern: String,
    pub search_bytes: Vec<u8>,
    pub search_regexp: Option<Regex>,
    pub last_search_char: Option<u8>,
    pub last_search_dir: i32,
    pub last_search_till: bool,
    pub quit_warn_remaining: i32,
    pub mouse_x: usize,
    pub mouse_y: usize,
    pub mouse_b: i32,
    pub paste_buffer: Vec<u8>,
    pub menu_open: bool,
    pub menu_x: usize,
    pub menu_y: usize,
    pub menu_selected: usize,
    pub is_dragging: bool,
    pub last_click_x: usize,
    pub last_click_y: usize,
    pub last_click_time: Option<Instant>,
    pub marks_x: [usize; 26],
    pub marks_y: [usize; 26],
    pub mark_set: [bool; 26],
    pub registers: Vec<Reg>,
    pub undo: Vec<UndoState>,
    pub redo: Vec<UndoState>,
    pub count_prefix: usize,
    pub last_rows: Vec<Row>,
    pub last_rowoff: usize,
    pub last_coloff: usize,
    pub last_change: Vec<i32>,
    pub recording_change: bool,
    pub current_change: Vec<i32>,
    pub key_buffer: Vec<i32>,
    pub selected_register: usize,
    pub pending_cmd: String,
    pub in_test: bool,
}

impl Editor {
    pub fn new() -> Self {
        Self {
            cx: 0,
            cy: 0,
            preferred: 0,
            rowoff: 0,
            coloff: 0,
            screen_rows: 0,
            screen_cols: 0,
            rows: Vec::new(),
            dirty: false,
            filename: String::new(),
            git_status: String::new(),
            statusmsg: String::new(),
            status_time: None,
            mode: Mode::Normal,
            sel_sx: 0,
            sel_sy: 0,
            search_pattern: String::new(),
            search_bytes: Vec::new(),
            search_regexp: None,
            last_search_char: None,
            last_search_dir: 1,
            last_search_till: false,
            quit_warn_remaining: 1,
            mouse_x: 0,
            mouse_y: 0,
            mouse_b: 0,
            paste_buffer: Vec::new(),
            menu_open: false,
            menu_x: 0,
            menu_y: 0,
            menu_selected: 0,
            is_dragging: false,
            last_click_x: 0,
            last_click_y: 0,
            last_click_time: None,
            marks_x: [0; 26],
            marks_y: [0; 26],
            mark_set: [false; 26],
            registers: vec![Reg::default(); 256],
            undo: Vec::new(),
            redo: Vec::new(),
            count_prefix: 0,
            last_rows: Vec::new(),
            last_rowoff: 0,
            last_coloff: 0,
            last_change: Vec::new(),
            recording_change: false,
            current_change: Vec::new(),
            key_buffer: Vec::new(),
            selected_register: '"' as usize,
            pending_cmd: String::new(),
            in_test: false,
        }
    }

    pub fn set_status(&mut self, msg: String) {
        self.statusmsg = msg;
        self.status_time = Some(Instant::now());
    }

    pub fn insert_row(&mut self, at: usize, s: Vec<u8>) {
        if at > self.rows.len() {
            return;
        }
        let mut row = Row::new(at, s);
        row.idx = at;
        self.rows.insert(at, row);
        for i in at + 1..self.rows.len() {
            self.rows[i].idx = i;
        }
        self.dirty = true;
    }

    pub fn del_row(&mut self, at: usize) {
        if at >= self.rows.len() {
            return;
        }
        self.rows.remove(at);
        for i in at..self.rows.len() {
            self.rows[i].idx = i;
        }
        self.dirty = true;
    }

    pub fn row_insert_char(&mut self, row_idx: usize, at: usize, c: u8) {
        if row_idx >= self.rows.len() {
            return;
        }
        let row = &mut self.rows[row_idx];
        let pos = if at > row.s.len() { row.s.len() } else { at };
        row.s.insert(pos, c);
        row.needs_highlight = true;
        self.dirty = true;
    }

    pub fn row_del_char(&mut self, row_idx: usize, at: usize) {
        if row_idx >= self.rows.len() {
            return;
        }
        let row = &mut self.rows[row_idx];
        if at >= row.s.len() {
            return;
        }
        row.s.remove(at);
        row.needs_highlight = true;
        self.dirty = true;
    }

    pub fn open_file<P: AsRef<Path>>(&mut self, path: P) -> io::Result<()> {
        let path = path.as_ref();
        self.filename = path.to_string_lossy().into_owned();

        if !path.exists() {
            self.rows.clear();
            self.dirty = false;
            self.set_status(format!("\"{}\" [New File]", self.filename));
            return Ok(());
        }

        let file = File::open(path)?;
        let reader = BufReader::new(file);
        self.rows.clear();

        for (idx, line) in reader.split(b'\n').enumerate() {
            let mut line = line?;
            if !line.is_empty() && line.last() == Some(&b'\r') {
                line.pop();
            }
            self.insert_row(idx, line);
        }

        self.dirty = false;
        self.update_git_status();
        Ok(())
    }

    pub fn save_file(&mut self) -> io::Result<()> {
        if self.filename.is_empty() {
            return Err(io::Error::new(io::ErrorKind::Other, "No filename"));
        }

        let mut file = File::create(&self.filename)?;
        for row in &self.rows {
            file.write_all(&row.s)?;
            file.write_all(b"\n")?;
        }

        self.dirty = false;
        let msg = format!("\"{}\" {}L written", self.filename, self.rows.len());
        self.set_status(msg);
        self.update_git_status();
        Ok(())
    }

    pub fn update_git_status(&mut self) {
        if self.filename.is_empty() { return; }
        
        let output = Command::new("git")
            .args(["status", "--porcelain", "-b"])
            .output();

        if let Ok(out) = output {
            let s = String::from_utf8_lossy(&out.stdout);
            if let Some(line) = s.lines().next() {
                if line.starts_with("## ") {
                    let mut branch = line[3..].split("...").next().unwrap_or("").to_string();
                    if s.lines().count() > 1 {
                        branch.push('*');
                    }
                    self.git_status = branch;
                }
            }
        }
    }

    pub fn set_search_pattern(&mut self, p: String) {
        self.search_pattern = p;
        if self.search_pattern.is_empty() {
            self.search_regexp = None;
            return;
        }
        let re = Regex::new(&format!("(?i){}", self.search_pattern));
        if let Ok(re) = re {
            self.search_regexp = Some(re);
        }
    }

    pub fn find_next(&mut self) {
        if self.search_regexp.is_none() { return; }
        let re = self.search_regexp.as_ref().unwrap().clone();
        
        let start_y = self.cy;
        let start_x = if self.cy < self.rows.len() && self.cx < self.rows[self.cy].s.len() {
            utf8_next_boundary(&self.rows[self.cy].s, self.cx)
        } else {
            0
        };

        for i in 0..self.rows.len() {
            let y = (start_y + i) % self.rows.len();
            let row = &self.rows[y].s;
            let from_x = if y == start_y { start_x } else { 0 };
            
            if from_x < row.len() {
                if let Some(m) = re.find(&row[from_x..]) {
                    self.cy = y;
                    self.cx = from_x + m.start();
                    self.preferred = self.cx;
                    return;
                }
            }
        }
    }

    pub fn find_prev(&mut self) {
        if self.search_regexp.is_none() { return; }
        let re = self.search_regexp.as_ref().unwrap().clone();

        let start_y = self.cy;
        let start_x = self.cx;

        for i in 0..self.rows.len() {
            let y = (start_y + self.rows.len() - i) % self.rows.len();
            let row = &self.rows[y].s;
            
            let search_limit = if y == start_y { start_x } else { row.len() };
            
            if search_limit > 0 {
                let matches: Vec<_> = re.find_iter(&row[..search_limit]).collect();
                if let Some(m) = matches.last() {
                    self.cy = y;
                    self.cx = m.start();
                    self.preferred = self.cx;
                    return;
                }
            }
        }
    }

    pub fn save_undo(&mut self) {
        let state = UndoState {
            rows: self.rows.clone(),
            cx: self.cx,
            cy: self.cy,
        };
        self.undo.push(state);
        self.redo.clear();
    }

    pub fn do_undo(&mut self) {
        if self.undo.is_empty() { return; }
        let current_state = UndoState {
            rows: self.rows.clone(),
            cx: self.cx,
            cy: self.cy,
        };
        self.redo.push(current_state);
        let last = self.undo.pop().unwrap();
        self.rows = last.rows;
        self.cx = last.cx;
        self.cy = last.cy;
        self.dirty = true;
    }

    pub fn do_redo(&mut self) {
        if self.redo.is_empty() { return; }
        let current_state = UndoState {
            rows: self.rows.clone(),
            cx: self.cx,
            cy: self.cy,
        };
        self.undo.push(current_state);
        let last = self.redo.pop().unwrap();
        self.rows = last.rows;
        self.cx = last.cx;
        self.cy = last.cy;
        self.dirty = true;
    }

    pub fn increment_number(&mut self, delta: i32) {
        if self.cy >= self.rows.len() { return; }
        let (i, j, n) = {
            let line = &self.rows[self.cy].s;
            let mut i = self.cx;
            while i < line.len() && !(line[i] >= b'0' && line[i] <= b'9') {
                if line[i] == b'-' && i + 1 < line.len() && line[i+1] >= b'0' && line[i+1] <= b'9' {
                    break;
                }
                i += 1;
            }
            if i >= line.len() { return; }
            let mut j = i;
            if line[j] == b'-' { j += 1; }
            while j < line.len() && line[j] >= b'0' && line[j] <= b'9' { j += 1; }
            
            let n_str = String::from_utf8_lossy(&line[i..j]);
            if let Ok(n) = n_str.parse::<i32>() {
                (i, j, n)
            } else {
                return;
            }
        };

        let new_n = n + delta;
        self.save_undo();
        
        let repl = new_n.to_string().into_bytes();
        let line = &self.rows[self.cy].s;
        let mut new_line = Vec::with_capacity(line.len() - (j - i) + repl.len());
        new_line.extend_from_slice(&line[..i]);
        new_line.extend_from_slice(&repl);
        new_line.extend_from_slice(&line[j..]);
        
        self.rows[self.cy] = self.rows[self.cy].duplicate();
        self.rows[self.cy].s = new_line;
        self.cx = (i + repl.len()).saturating_sub(1);
        self.preferred = self.cx;
        self.rows[self.cy].needs_highlight = true;
        self.dirty = true;
    }

    pub fn set_clipboard(&self, text: &[u8]) {
        if let Ok(mut cb) = Clipboard::new() {
            let _ = cb.set_text(String::from_utf8_lossy(text).into_owned());
        }
    }

    pub fn get_clipboard(&self) -> Option<Vec<u8>> {
        if let Ok(mut cb) = Clipboard::new() {
            if let Ok(text) = cb.get_text() {
                return Some(text.into_bytes());
            }
        }
        None
    }

    pub fn yoink(&mut self, mut sx: usize, mut sy: usize, mut ex: usize, mut ey: usize, is_line: bool) {
        if sy > ey || (sy == ey && sx > ex) {
            std::mem::swap(&mut sx, &mut ex);
            std::mem::swap(&mut sy, &mut ey);
        }
        
        let mut b = Vec::new();
        if is_line {
            for i in sy..=ey {
                if i < self.rows.len() {
                    b.extend_from_slice(&self.rows[i].s);
                    b.push(b'\n');
                }
            }
        } else if sy == ey && sy < self.rows.len() {
            let r = &self.rows[sy].s;
            if sx < r.len() {
                let end = (ex + 1).min(r.len());
                if sx < end {
                    b.extend_from_slice(&r[sx..end]);
                }
            }
        } else {
            for i in sy..=ey {
                if i >= self.rows.len() { break; }
                let r = &self.rows[i].s;
                if i == sy {
                    if sx < r.len() { b.extend_from_slice(&r[sx..]); }
                    b.push(b'\n');
                } else if i == ey {
                    let end = (ex + 1).min(r.len());
                    if ex < r.len() { b.extend_from_slice(&r[..end]); }
                } else {
                    b.extend_from_slice(r);
                    b.push(b'\n');
                }
            }
        }

        let reg_name = self.selected_register;
        if reg_name == '_' as usize { return; }
        
        self.registers[reg_name] = Reg { s: b.clone(), is_line };
        if reg_name == '"' as usize {
            self.set_clipboard(&b);
        }
    }

    pub fn delete_range(&mut self, mut sx: usize, mut sy: usize, mut ex: usize, mut ey: usize) {
        if sy > ey || (sy == ey && sx > ex) {
            std::mem::swap(&mut sx, &mut ex);
            std::mem::swap(&mut sy, &mut ey);
        }
        self.save_undo();
        if sy == ey {
            if sy < self.rows.len() {
                self.rows[sy] = self.rows[sy].duplicate();
                let r = &mut self.rows[sy];
                let end = (ex + 1).min(r.s.len());
                if sx < r.s.len() && sx < end {
                    r.s.drain(sx..end);
                    r.needs_highlight = true;
                }
            }
        } else {
            if sy < self.rows.len() {
                self.rows[sy] = self.rows[sy].duplicate();
                let first_part = self.rows[sy].s[..sx].to_vec();
                let mut last_part = Vec::new();
                if ey < self.rows.len() {
                    let last_row_s = &self.rows[ey].s;
                    if ex + 1 < last_row_s.len() {
                        last_part = last_row_s[ex + 1..].to_vec();
                    }
                }
                self.rows[sy].s = [first_part, last_part].concat();
                self.rows[sy].needs_highlight = true;
                for _ in 0..(ey - sy) {
                    self.del_row(sy + 1);
                }
            }
        }
        self.cy = sy;
        self.cx = sx;
        if self.cy >= self.rows.len() { self.cy = self.rows.len().saturating_sub(1); }
        self.preferred = self.cx;
    }

    pub fn paste(&mut self) {
        let clip_data = self.get_clipboard();
        if let Some(clip) = clip_data {
            let is_line = clip.contains(&b'\n');
            self.registers['"' as usize] = Reg { s: clip, is_line };
        }
        
        let (s, is_line) = {
            let r = &self.registers['"' as usize];
            if r.s.is_empty() { return; }
            (r.s.clone(), r.is_line)
        };

        self.save_undo();
        if is_line {
            let s_str = String::from_utf8_lossy(&s);
            let mut at = self.cy + 1;
            for ln in s_str.lines() {
                if ln.is_empty() { continue; }
                self.insert_row(at, ln.as_bytes().to_vec());
                at += 1;
            }
        } else {
            for &c in &s {
                if c == b'\n' { self.insert_newline(); }
                else { self.insert_char(c); }
            }
        }
    }

    pub fn select_word(&mut self) {
        if self.cy >= self.rows.len() || self.rows[self.cy].s.is_empty() { return; }
        let r = &self.rows[self.cy].s;
        let mut sx = self.cx;
        let mut ex = self.cx;
        while sx > 0 && is_word_char(r[sx - 1]) { sx -= 1; }
        while ex < r.len().saturating_sub(1) && is_word_char(r[ex + 1]) { ex += 1; }
        self.mode = Mode::Visual;
        self.sel_sy = self.cy;
        self.sel_sx = sx;
        self.cx = ex;
    }

    pub fn find_char(&mut self, c: u8, direction: i32, till: bool) -> bool {
        if self.rows.is_empty() || self.cy >= self.rows.len() { return false; }
        self.last_search_char = Some(c);
        self.last_search_dir = direction;
        self.last_search_till = till;
        
        let line = &self.rows[self.cy].s;
        if line.is_empty() { return false; }
        
        let mut x = self.cx;
        if direction > 0 {
            x += 1;
            while x < line.len() {
                if line[x] == c {
                    if till { x = x.saturating_sub(1); }
                    self.cx = x;
                    self.preferred = x;
                    return true;
                }
                x += 1;
            }
        } else {
            if x == 0 { return false; }
            x -= 1;
            loop {
                if line[x] == c {
                    if till { x += 1; if x >= line.len() { x = line.len().saturating_sub(1); } }
                    self.cx = x;
                    self.preferred = x;
                    return true;
                }
                if x == 0 { break; }
                x -= 1;
            }
        }
        false
    }

    pub fn repeat_char_search(&mut self, reverse: bool) {
        if let Some(c) = self.last_search_char {
            let mut dir = self.last_search_dir;
            if reverse { dir = -dir; }
            self.find_char(c, dir, self.last_search_till);
        }
    }

    pub fn handle_substitute(&mut self, cmd: &str) {
        let mut all_lines = false;
        let mut s_cmd = cmd;
        if cmd.starts_with('%') {
            all_lines = true;
            s_cmd = &cmd[1..];
        }
        if !s_cmd.starts_with('s') { return; }
        let rest = &s_cmd[1..];
        if rest.len() < 3 { self.set_status("Invalid substitute command".into()); return; }
        
        let delimiter = rest.chars().next().unwrap();
        let parts: Vec<&str> = rest[1..].split(delimiter).collect();
        if parts.len() < 2 { self.set_status("Invalid substitute command".into()); return; }
        
        let pattern = parts[0];
        let replacement = parts[1];
        let flags = if parts.len() > 2 { parts[2] } else { "" };
        let global = flags.contains('g');
        
        let start_row = if all_lines { 0 } else { self.cy };
        let end_row = if all_lines { self.rows.len().saturating_sub(1) } else { self.cy };
        
        if start_row >= self.rows.len() { return; }
        
        let re = Regex::new(&format!("(?i){}", pattern));
        if let Err(e) = re { self.set_status(format!("Invalid regex: {}", e)); return; }
        let re = re.unwrap();
        
        self.save_undo();
        let mut made_changes = false;
        let repl_bytes = replacement.as_bytes();
        for y in start_row..=end_row {
            let line = &self.rows[y].s;
            let new_line = if global {
                re.replace_all(line, repl_bytes).into_owned()
            } else {
                re.replace(line, repl_bytes).into_owned()
            };
            
            if new_line != self.rows[y].s {
                self.rows[y] = self.rows[y].duplicate();
                self.rows[y].s = new_line;
                self.rows[y].needs_highlight = true;
                made_changes = true;
            }
        }
        if made_changes {
            self.dirty = true;
            self.set_status("Substitutions complete".into());
        } else {
            self.set_status("Pattern not found".into());
        }
    }

    pub fn match_bracket(&mut self) {
        if self.cy >= self.rows.len() { return; }
        let line = &self.rows[self.cy].s;
        if line.is_empty() { return; }
        let c = line[self.cx];
        let (open, close, dir) = match c {
            b'(' => (b'(', b')', 1),
            b'[' => (b'[', b']', 1),
            b'{' => (b'{', b'}', 1),
            b')' => (b')', b'(', -1),
            b']' => (b']', b'[', -1),
            b'}' => (b'}', b'{', -1),
            _ => { return; }
        };
        
        let mut depth = 0;
        let mut y = self.cy;
        let mut x = self.cx;
        
        loop {
            if line[x] == open { depth += 1; }
            else if line[x] == close {
                depth -= 1;
                if depth == 0 {
                    self.cy = y;
                    self.cx = x;
                    self.preferred = x;
                    return;
                }
            }
            
            if dir > 0 {
                x += 1;
                while y < self.rows.len() && x >= self.rows[y].s.len() {
                    y += 1; x = 0;
                }
            } else {
                if x == 0 {
                    if y == 0 { break; }
                    y -= 1; x = self.rows[y].s.len().saturating_sub(1);
                } else {
                    x -= 1;
                }
            }
            if y >= self.rows.len() { break; }
        }
    }

    pub fn move_cursor(&mut self, key: crossterm::event::KeyCode) {
        match key {
            crossterm::event::KeyCode::Left => {
                if self.cx > 0 {
                    self.cx = utf8_prev_boundary(&self.rows[self.cy].s, self.cx);
                } else if self.cy > 0 {
                    self.cy -= 1;
                    self.cx = self.rows[self.cy].s.len();
                    if self.mode != Mode::Insert && self.cx > 0 {
                        self.cx = utf8_prev_boundary(&self.rows[self.cy].s, self.cx);
                    }
                }
            }
            crossterm::event::KeyCode::Right => {
                if self.cy < self.rows.len() && self.cx < self.rows[self.cy].s.len() {
                    self.cx = utf8_next_boundary(&self.rows[self.cy].s, self.cx);
                } else if self.cy < self.rows.len() && self.mode == Mode::Insert && self.cy < self.rows.len() - 1 {
                    self.cy += 1;
                    self.cx = 0;
                }
            }
            crossterm::event::KeyCode::Up => {
                if self.cy > 0 {
                    self.cy -= 1;
                }
            }
            crossterm::event::KeyCode::Down => {
                if self.cy < self.rows.len().saturating_sub(1) {
                    self.cy += 1;
                }
            }
            _ => {}
        }

        if self.rows.is_empty() {
            self.cx = 0;
            self.cy = 0;
            return;
        }

        if self.cy >= self.rows.len() {
            self.cy = self.rows.len() - 1;
        }

        let mut limit = self.rows[self.cy].s.len();
        if self.mode != Mode::Insert && limit > 0 {
            limit = utf8_prev_boundary(&self.rows[self.cy].s, limit);
        }

        match key {
            crossterm::event::KeyCode::Up | crossterm::event::KeyCode::Down => {
                if self.preferred > limit {
                    self.cx = limit;
                } else {
                    self.cx = self.preferred;
                }
            }
            _ => {
                if self.cx > limit {
                    self.cx = limit;
                }
                self.preferred = self.cx;
            }
        }
    }

    pub fn insert_char(&mut self, c: u8) {
        if self.cy == self.rows.len() {
            self.insert_row(self.rows.len(), Vec::new());
        }
        self.row_insert_char(self.cy, self.cx, c);
        self.cx = utf8_next_boundary(&self.rows[self.cy].s, self.cx);
        self.preferred = self.cx;
    }

    pub fn insert_newline(&mut self) {
        if self.cx == 0 {
            self.insert_row(self.cy, Vec::new());
        } else {
            let remainder = self.rows[self.cy].s[self.cx..].to_vec();
            self.rows[self.cy].s.truncate(self.cx);
            self.rows[self.cy].needs_highlight = true;
            self.insert_row(self.cy + 1, remainder);
        }
        self.cy += 1;
        self.cx = 0;
        self.preferred = 0;
    }

    pub fn del_char(&mut self) {
        if self.cy == self.rows.len() || (self.cx == 0 && self.cy == 0) {
            return;
        }
        if self.cx > 0 {
            let prev = utf8_prev_boundary(&self.rows[self.cy].s, self.cx);
            self.row_del_char(self.cy, prev);
            self.cx = prev;
        } else {
            let prev_row_idx = self.cy - 1;
            let current_row_s = self.rows[self.cy].s.clone();
            self.cx = self.rows[prev_row_idx].s.len();
            self.rows[prev_row_idx].s.extend(current_row_s);
            self.rows[prev_row_idx].needs_highlight = true;
            self.del_row(self.cy);
            self.cy -= 1;
        }
        self.preferred = self.cx;
    }

    pub fn gutter_width(&self) -> usize {
        if self.filename.is_empty() && self.rows.is_empty() { return 0; }
        let mut n = std::cmp::max(1, self.rows.len());
        let mut w = 1;
        while n >= 10 {
            n /= 10;
            w += 1;
        }
        w
    }

    pub fn get_rx(&self) -> usize {
        if self.cy >= self.rows.len() { return 0; }
        let row = &self.rows[self.cy].s;
        let mut rx = 0;
        let mut i = 0;
        while i < self.cx && i < row.len() {
            if row[i] == b'\t' {
                rx += 8 - (rx % 8);
                i += 1;
            } else {
                let (_, n) = decode_utf8_rune(&row[i..]);
                rx += 1;
                i += n;
            }
        }
        rx
    }

    pub fn scroll(&mut self) {
        let g = self.gutter_width();
        let gcols = if g > 0 { g + 1 } else { 0 };
        let text_cols = self.screen_cols.saturating_sub(gcols);
        let text_cols = if text_cols < 1 { 1 } else { text_cols };

        if self.cy < self.rowoff {
            self.rowoff = self.cy;
        }
        if self.cy >= self.rowoff + self.screen_rows {
            self.rowoff = self.cy - self.screen_rows + 1;
        }
        if self.cx < self.coloff {
            self.coloff = self.cx;
        }
        if self.cx >= self.coloff + text_cols {
            self.coloff = self.cx - text_cols + 1;
        }
    }

    pub fn move_line_start(&mut self) {
        self.cx = 0;
        self.preferred = 0;
    }

    pub fn move_line_end(&mut self) {
        if self.cy < self.rows.len() {
            self.cx = self.rows[self.cy].s.len();
            if self.mode != Mode::Insert && self.cx > 0 {
                self.cx = utf8_prev_boundary(&self.rows[self.cy].s, self.cx);
            }
            self.preferred = self.cx;
        }
    }

    pub fn move_first_non_whitespace(&mut self) {
        if self.cy < self.rows.len() {
            let row = &self.rows[self.cy].s;
            let mut col = 0;
            while col < row.len() && (row[col] == b' ' || row[col] == b'\t') {
                col += 1;
            }
            self.cx = col;
            self.preferred = self.cx;
        }
    }

    pub fn move_word_forward(&mut self) {
        if self.rows.is_empty() { return; }
        let mut r = self.cy;
        let mut c = self.cx;
        
        while r < self.rows.len() {
            let line = &self.rows[r].s;
            if c < line.len() {
                if is_word_char(line[c]) {
                    while c < line.len() && is_word_char(line[c]) { c += 1; }
                } else {
                    while c < line.len() && !is_word_char(line[c]) && line[c] != b' ' && line[c] != b'\t' { c += 1; }
                }
            }
            while c < line.len() && (line[c] == b' ' || line[c] == b'\t') { c += 1; }
            if c < line.len() {
                self.cy = r;
                self.cx = c;
                self.preferred = c;
                return;
            }
            r += 1;
            c = 0;
        }
    }

    pub fn move_word_backward(&mut self) {
        if self.rows.is_empty() { return; }
        if self.cx == 0 && self.cy == 0 { return; }
        
        let mut r = self.cy;
        let mut c = self.cx.saturating_sub(1);

        while r < self.rows.len() {
            let line = &self.rows[r].s;
            while c < line.len() && (line[c] == b' ' || line[c] == b'\t') {
                if c == 0 { break; }
                c -= 1;
            }
            if c < line.len() && (line[c] != b' ' && line[c] != b'\t') {
                if is_word_char(line[c]) {
                    while c > 0 && is_word_char(line[c-1]) { c -= 1; }
                } else {
                    while c > 0 && !is_word_char(line[c-1]) && line[c-1] != b' ' && line[c-1] != b'\t' { c -= 1; }
                }
                self.cy = r;
                self.cx = c;
                self.preferred = c;
                return;
            }
            if r == 0 { break; }
            r -= 1;
            c = self.rows[r].s.len().saturating_sub(1);
        }
    }

    pub fn move_next_paragraph(&mut self) {
        if self.rows.is_empty() { return; }
        let mut y = self.cy + 1;
        while y < self.rows.len() {
            if self.rows[y].s.is_empty() { break; }
            y += 1;
        }
        self.cy = y.min(self.rows.len().saturating_sub(1));
        self.cx = 0;
        self.preferred = 0;
    }

    pub fn move_prev_paragraph(&mut self) {
        if self.rows.is_empty() { return; }
        if self.cy == 0 { return; }
        let mut y = self.cy - 1;
        while y > 0 {
            if self.rows[y].s.is_empty() { break; }
            y -= 1;
        }
        self.cy = y;
        self.cx = 0;
        self.preferred = 0;
    }

    pub fn move_file_start(&mut self) {
        self.cy = 0;
        self.cx = 0;
        self.preferred = 0;
    }

    pub fn move_file_end(&mut self) {
        if !self.rows.is_empty() {
            self.cy = self.rows.len() - 1;
            self.move_line_end();
        }
    }
}

fn is_word_char(c: u8) -> bool {
    c.is_ascii_alphanumeric() || c == b'_'
}

// UTF-8 Helpers
fn decode_utf8_rune(s: &[u8]) -> (char, usize) {
    if s.is_empty() { return ('\0', 0); }
    let first = s[0];
    if first & 0x80 == 0 { return (first as char, 1); }
    let n = if first & 0xE0 == 0xC0 { 2 }
            else if first & 0xF0 == 0xE0 { 3 }
            else if first & 0xF8 == 0xF0 { 4 }
            else { 1 };
    if n > s.len() { return (first as char, 1); }
    let s_str = std::str::from_utf8(&s[..n]).unwrap_or("");
    (s_str.chars().next().unwrap_or(first as char), n)
}

pub fn utf8_prev_boundary(s: &[u8], mut idx: usize) -> usize {
    if idx == 0 { return 0; }
    if idx > s.len() { idx = s.len(); }
    idx -= 1;
    while idx > 0 && (s[idx] & 0xC0) == 0x80 {
        idx -= 1;
    }
    idx
}

pub fn utf8_next_boundary(s: &[u8], mut idx: usize) -> usize {
    if idx >= s.len() { return s.len(); }
    while idx < s.len() && (s[idx] & 0xC0) == 0x80 {
        idx += 1;
    }
    if idx >= s.len() { return s.len(); }
    
    // Simple next boundary logic
    let first_byte = s[idx];
    let n = if first_byte & 0x80 == 0 {
        1
    } else if first_byte & 0xE0 == 0xC0 {
        2
    } else if first_byte & 0xF0 == 0xE0 {
        3
    } else if first_byte & 0xF8 == 0xF0 {
        4
    } else {
        1
    };
    
    if idx + n > s.len() { s.len() } else { idx + n }
}

pub fn utf8_snap_boundary(s: &[u8], mut idx: usize) -> usize {
    if idx == 0 { return 0; }
    if idx >= s.len() { return s.len(); }
    while idx > 0 && (s[idx] & 0xC0) == 0x80 {
        idx -= 1;
    }
    idx
}
