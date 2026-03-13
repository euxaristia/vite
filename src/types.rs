#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Mode {
    Normal,
    Insert,
    Visual,
    VisualLine,
    Replace,
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Highlight {
    Normal,
    Comment,
    Keyword1,
    Keyword2,
    String,
    Number,
    Match,
    MatchCursor,
    Visual,
}

#[derive(Clone, Debug, Default)]
pub struct Reg {
    pub s: Vec<u8>,
    pub is_line: bool,
}
