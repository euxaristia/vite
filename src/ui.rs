use crossterm::{
    cursor,
    queue,
    style::{Color, Print, SetBackgroundColor, SetForegroundColor},
    terminal::{self, Clear, ClearType},
};
use std::io::{stdout, Result, Write};

use crate::types::{Highlight, Mode};
use crate::editor::{Editor, utf8_snap_boundary};
use crate::syntax;

pub fn enable_raw_mode() -> Result<()> {
    terminal::enable_raw_mode()
}

fn highlight_to_color(hl: Highlight) -> Color {
    match hl {
        Highlight::Normal => Color::White,
        Highlight::Comment => Color::Green,
        Highlight::Keyword1 => Color::Yellow,
        Highlight::Keyword2 => Color::Cyan,
        Highlight::String => Color::Magenta,
        Highlight::Number => Color::Red,
        Highlight::Match => Color::Blue,
        Highlight::MatchCursor => Color::Yellow,
        Highlight::Visual => Color::Grey,
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
    
    let cx = (editor.cx.saturating_sub(editor.coloff) + gcols) as u16; 
    let cy = editor.cy.saturating_sub(editor.rowoff) as u16;

    queue!(stdout, cursor::MoveTo(cx, cy), cursor::Show)?;
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

    for y in 0..editor.screen_rows {
        let filerow = y + editor.rowoff;
        if filerow >= editor.rows.len() {
            if editor.rows.len() == 0 && y >= editor.screen_rows / 3 && y < editor.screen_rows / 3 + WELCOME_LINES.len() {
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
            // Draw gutter
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
            let len = row.s.len().saturating_sub(start);
            let display_len = if len > text_cols { text_cols } else { len };

            if start < row.s.len() {
                let end = start + display_len;
                let visible_s = &row.s[start..end];
                let visible_hl = &row.hl[start..end];
                
                for (&ch, &hl) in visible_s.iter().zip(visible_hl.iter()) {
                    queue!(stdout, SetForegroundColor(highlight_to_color(hl)))?;
                    if ch == b'\t' {
                        queue!(stdout, Print(" "))?; // Simplified tab for now
                    } else {
                        queue!(stdout, Print(ch as char))?;
                    }
                }
                queue!(stdout, SetForegroundColor(Color::Reset))?;
            }
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
        SetBackgroundColor(Color::White),
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
            Mode::Normal => String::from("-- NORMAL --"),
            Mode::Insert => String::from("-- INSERT --"),
            Mode::Visual => String::from("-- VISUAL --"),
            Mode::VisualLine => String::from("-- VISUAL LINE --"),
        };
    }
    
    let len = std::cmp::min(msg.len(), editor.screen_cols);
    queue!(stdout, Print(&msg[..len]))?;
    Ok(())
}
