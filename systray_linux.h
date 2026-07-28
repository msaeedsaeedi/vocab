#ifndef SYSTRAY_LINUX_H
#define SYSTRAY_LINUX_H

int setup_indicator(void);
void set_tray_icon(const char *data, int len);
void add_menu_item(int id, const char *label);
void add_check_item(int id, const char *label, int checked);
void add_separator(void);
void set_item_checked(int id, int checked);
void remove_indicator(void);

#endif
