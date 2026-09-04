package models

// Menu type enum values used by sys_menu.menu_type.
const (
	// Directory is a menu directory (a grouping node with no page of its own).
	Directory string = "M"
	// Menu is a page menu entry.
	Menu string = "C"
	// Button is a button-level permission entry.
	Button string = "F"
)
