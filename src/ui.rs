use crossterm::{
    cursor,
    queue,
    style::{Color, Print, SetBackgroundColor, SetForegroundColor},
    terminal::{self, Clear, ClearType},
};
use std::io::{stdout, Result, Write};

use crate::types::{Highlight, Mode};
use crate::editor::{Editor, utf8_snap_boundary, display_width_bytes, rune_display_width};
use crate::syntax;

pub fn enable_raw_mode() -> Result<()> {
    terminal::enable_raw_mode()
}

fn highlight_to_color(hl: Highlight) -> (Color, Color) {
    match hl {
        Highlight::Normal => (Color::White, Color::Reset),
        Highlight::Comment => (Color::Green, Color::Reset),
        Highlight::Keyword1 => (Color::Yellow, Color::Reset),
        Highlight::Keyword2 => (Color::Cyan, Color::Reset),
        Highlight::String => (Color::Magenta, Color::Reset),
        Highlight::Number => (Color::Red, Color::Reset),
        Highlight::Match => (Color::White, Color::DarkBlue),
        Highlight::MatchCursor => (Color::Black, Color::Yellow),
        Highlight::Visual => (Color::White, Color::DarkGrey),
    }
}

pub fn disable_raw_mode() -> Result<()> {
    terminal::disable_raw_mode()
}

pub fn get_terminal_size() -> Result<(usize, usize)> {
    let (cols, rows) = terminal::size()?;
    Ok((cols as usize, rows as usize))
}

pub fn refresh_screen(editor: &mut Editor) -> Result<()> {
    editor.scroll();
    syntax::update_all_syntax(&editor.filename, &editor.search_regexp, &mut editor.rows, false);
    let mut stdout = stdout();

    queue!(stdout, cursor::Hide, cursor::MoveTo(0, 0))?;

    draw_rows(editor, &mut stdout)?;
    draw_status_bar(editor, &mut stdout)?;
    draw_message_bar(editor, &mut stdout)?;

    let g = editor.gutter_width();
    let gcols = if g > 0 { g + 1 } else { 0 };
    
    let mut cur_y = (editor.cy.saturating_sub(editor.rowoff)) as u16;
    let mut cur_x = gcols as u16;

    if editor.cy < editor.rows.len() {
        let row = &editor.rows[editor.cy].s;
        let start = utf8_snap_boundary(row, editor.coloff);
        let end = editor.cx.min(row.len());
        if end > start {
            cur_x += display_width_bytes(&row[start..end], gcols) as u16;
        }
    }

    if !editor.statusmsg.is_empty() && editor.statusmsg.starts_with(':') {
        cur_y = (editor.screen_rows + 1) as u16;
        cur_x = editor.statusmsg.len() as u16;
    }

    queue!(stdout, cursor::MoveTo(cur_x, cur_y), cursor::Show)?;
    stdout.flush()?;
    Ok(())
}

const WELCOME_LINES: &[&str] = &[
    "VIDERE v0.1.0",
    "",
    "videre is open source and freely distributable",
    "https://github.com/euxaristia/videre",
    "",
    "type  :q<Enter>               to exit         ",
    "type  :wq<Enter>              save and exit   ",
    "",
    "Maintainer: euxaristia",
];

fn draw_rows(editor: &mut Editor, stdout: &mut impl Write) -> Result<()> {
    let g = editor.gutter_width();
    let gcols = if g > 0 { g + 1 } else { 0 };
    let text_cols = editor.screen_cols.saturating_sub(gcols);

    let has_selection = editor.mode == Mode::Visual || editor.mode == Mode::VisualLine;
    let line_selection = editor.mode == Mode::VisualLine;
    let (mut sy, mut ey, mut sx, mut ex) = (0, 0, 0, 0);
    if has_selection {
        sy = editor.sel_sy; ey = editor.cy;
        sx = editor.sel_sx; ex = editor.cx;
        if sy > ey || (sy == ey && sx > ex) {
            std::mem::swap(&mut sy, &mut ey);
            std::mem::swap(&mut sx, &mut ex);
        }
    }

    for y in 0..editor.screen_rows {
        let filerow = y + editor.rowoff;
        if filerow >= editor.rows.len() {
            if editor.rows.is_empty() && y >= editor.screen_rows / 3 && y < editor.screen_rows / 3 + WELCOME_LINES.len() {
                queue!(stdout, SetForegroundColor(Color::DarkGrey), Print("~"), SetForegroundColor(Color::Reset))?;
                
                let msg = WELCOME_LINES[y - editor.screen_rows / 3];
                let padding = text_cols.saturating_sub(msg.len()) / 2;
                for _ in 0..padding {
                    queue!(stdout, Print(" "))?;
                }
                let display_len = std::cmp::min(msg.len(), text_cols.saturating_sub(padding));
                queue!(stdout, Print(&msg[..display_len]))?;
            } else {
                queue!(stdout, SetForegroundColor(Color::DarkGrey), Print("~"), SetForegroundColor(Color::Reset))?;
            }
        } else {
            if g > 0 {
                let line_num = (filerow + 1).to_string();
                let padding = g.saturating_sub(line_num.len());
                queue!(stdout, SetForegroundColor(Color::DarkGrey))?;
                for _ in 0..padding {
                    queue!(stdout, Print(" "))?;
                }
                queue!(stdout, Print(line_num))?;
                queue!(stdout, Print(" "), SetForegroundColor(Color::Reset))?;
            }

            let row = &editor.rows[filerow];
            let start = utf8_snap_boundary(&row.s, editor.coloff);
            let row_in_selection = has_selection && filerow >= sy && filerow <= ey;

            let mut col = 0;
            let mut i = start;
            while i < row.s.len() && col < text_cols {
                let (r, n) = crate::editor::decode_utf8_rune(&row.s[i..]);
                if n == 0 { break; }

                let mut hl = row.hl[i];
                // Check if current search match is under cursor
                if hl == Highlight::Match
                    && let Some(re) = &editor.search_regexp
                        && let Some(m) = re.find(&row.s)
                            && filerow == editor.cy && i >= m.start() && i < m.end() && editor.cx >= m.start() && editor.cx < m.end() {
                                hl = Highlight::MatchCursor;
                            }

                let (fg, mut bg) = highlight_to_color(hl);

                if row_in_selection {
                    let x = i;
                    let sel = if line_selection { true }
                            else if sy == ey { x >= sx && x <= ex }
                            else if filerow == sy { x >= sx }
                            else if filerow == ey { x <= ex }
                            else { true };
                    if sel { bg = Color::DarkGrey; }
                }

                queue!(stdout, SetForegroundColor(fg), SetBackgroundColor(bg))?;
                if r == '\t' {
                    let tab_w = 8 - ((gcols + col) % 8);
                    for _ in 0..tab_w {
                        if col < text_cols {
                            queue!(stdout, Print(" "))?;
                            col += 1;
                        }
                    }
                } else {
                    let w = rune_display_width(r);
                    if col + w <= text_cols {
                        queue!(stdout, Print(r))?;
                        col += w;
                    } else {
                        break;
                    }
                }
                i += n;
            }
            queue!(stdout, SetForegroundColor(Color::Reset), SetBackgroundColor(Color::Reset))?;
        }
        
        queue!(stdout, Clear(ClearType::UntilNewLine))?;
        queue!(stdout, Print("\r\n"))?;
    }
    Ok(())
}

fn draw_status_bar(editor: &Editor, stdout: &mut impl Write) -> Result<()> {
    let mut left = if editor.filename.is_empty() {
        String::from(" [No Name]")
    } else {
        format!(" {}", editor.filename)
    };
    if editor.dirty {
        left.push_str(" [+]");
    }
    if !editor.git_status.is_empty() {
        left.push_str(&format!(" [{}]", editor.git_status));
    }

    let pos = if editor.rows.is_empty() {
        String::from("All")
    } else if editor.rowoff == 0 {
        String::from("Top")
    } else if editor.rowoff + editor.screen_rows >= editor.rows.len() {
        String::from("Bot")
    } else {
        let percent = (editor.rowoff * 100) / (editor.rows.len() - editor.screen_rows).max(1);
        format!("{}%", percent)
    };

    let rx = editor.get_rx();
    let loc = format!("{}:{},{}-{}", editor.filename, editor.cy + 1, editor.cx + 1, rx + 1);
    let right = format!(" {} {} ", loc, pos);

    queue!(
        stdout,
        SetBackgroundColor(Color::Grey),
        SetForegroundColor(Color::Black)
    )?;

    let left_len = std::cmp::min(left.len(), editor.screen_cols.saturating_sub(right.len()));
    queue!(stdout, Print(&left[..left_len]))?;
    
    let padding = editor.screen_cols.saturating_sub(left_len).saturating_sub(right.len());
    for _ in 0..padding {
        queue!(stdout, Print(" "))?;
    }
    
    if editor.screen_cols >= left_len + right.len() {
        queue!(stdout, Print(right))?;
    }
    
    queue!(
        stdout,
        SetBackgroundColor(Color::Reset),
        SetForegroundColor(Color::Reset),
        Print("\r\n")
    )?;
    Ok(())
}

fn draw_message_bar(editor: &Editor, stdout: &mut impl Write) -> Result<()> {
    queue!(stdout, Clear(ClearType::UntilNewLine))?;
    
    let mut msg = editor.statusmsg.clone();
    if msg.is_empty() {
        msg = match editor.mode {
            Mode::Insert => String::from("-- INSERT --"),
            Mode::Visual => String::from("-- VISUAL --"),
            Mode::VisualLine => String::from("-- VISUAL LINE --"),
            Mode::Replace => String::from("-- REPLACE --"),
            _ => String::new(),
        };
    }
    
    let len = std::cmp::min(msg.len(), editor.screen_cols);
    queue!(stdout, Print(&msg[..len]))?;
    Ok(())
}
