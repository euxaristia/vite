use std::fs::File;
use std::io::{self, BufRead, BufReader, Write};
use std::path::Path;
use std::time::Instant;
use regex::bytes::Regex;

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
            selected_register: 0,
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
        let row = Row::new(at, s);
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
        Ok(())
    }

    pub fn save_file(&mut self) -> io::Result<()> {
        if self.filename.is_empty() {
            // In a real implementation, we'd prompt here.
            // For now, return an error if no filename.
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
        Ok(())
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
        let re = self.search_regexp.as_ref().unwrap();
        
        let start_y = self.cy;
        let start_x = if self.cx < self.rows[self.cy].s.len() {
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
        let re = self.search_regexp.as_ref().unwrap();

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
