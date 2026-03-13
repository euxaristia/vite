use std::env;
use std::process;
use videre::{Editor, ui, input, init_editor};

const VERSION_BANNER: &str = r#" ┌──────────────────────────────────────────────────────────────┐
 │                                                              │
 │   __     __           ____  U _____ u   ____    U _____ u    │
 │  \ \   /"/u  ___    |  _"\ \| ___"|/U |  _"\ u \| ___"|/     │
 │   \ \ / //  |_"_|  /| | | | |  _|"   \| |_) |/  |  _|"       │
 │   /\ V /_,-. | |   U| |_| |\| |___    |  _ <    | |___       │
 │  U  \_/-(_/U/| |\u  |____/ u|_____|   |_| \_\   |_____|      │
 │    //   .-,_|___|_,-.|||_   <<   >>   //   \\_  <<   >>      │
 │   (__)   \_)-' '-(_/(__)_) (__) (__) (__)  (__)(__) (__)     │
 │                                                              │
 └──────────────────────────────────────────────────────────────┘"#;

fn main() {
    let args: Vec<String> = env::args().collect();
    for arg in args.iter().skip(1) {
        if arg == "--version" || arg == "-V" {
            println!("{}", VERSION_BANNER);
            let version_str = "videre v0.1.0";
            let padding = 64_usize.saturating_sub(version_str.len()) / 2;
            println!("{}{}", " ".repeat(padding), version_str);
            process::exit(0);
        }
    }

    let mut editor = init_editor();

    if args.len() >= 2 {
        let _ = editor.open_file(&args[1]);
        if let Ok((cols, rows)) = ui::get_terminal_size() {
            editor.screen_cols = cols;
            editor.screen_rows = rows.saturating_sub(2);
        }
    }

    if let Err(e) = ui::enable_raw_mode() {
        eprintln!("Failed to enable raw mode: {}", e);
        process::exit(1);
    }

    let result = run_editor(&mut editor);

    let _ = ui::disable_raw_mode();
    let _ = crossterm::execute!(std::io::stdout(), crossterm::terminal::Clear(crossterm::terminal::ClearType::All));

    if let Err(e) = result {
        eprintln!("Error: {}", e);
        process::exit(1);
    }
}

fn run_editor(editor: &mut Editor) -> std::io::Result<()> {
    loop {
        ui::refresh_screen(editor)?;
        if input::process_keypress(editor)? {
            break;
        }
    }
    Ok(())
}
