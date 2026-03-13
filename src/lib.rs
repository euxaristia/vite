pub mod types;
pub mod editor;
pub mod ui;
pub mod input;
pub mod syntax;

pub use editor::Editor;

pub fn init_editor() -> Editor {
    let mut editor = Editor::new();
    if let Ok((cols, rows)) = ui::get_terminal_size() {
        editor.screen_cols = cols;
        editor.screen_rows = rows.saturating_sub(2);
    }
    editor
}
