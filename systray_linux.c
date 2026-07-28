#include "systray_linux.h"
#include <libappindicator/app-indicator.h>
#include <gtk/gtk.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <stdio.h>
#include <errno.h>
#include <limits.h>

static AppIndicator *indicator;
static GtkWidget *menu;

typedef struct {
	int id;
	GtkWidget *widget;
} MenuEntry;

static MenuEntry entries[64];
static int entry_count = 0;
static char temp_icon_path[PATH_MAX] = "";

extern void goMenuItemClicked(int id);

static MenuEntry *find_entry(int id) {
	for (int i = 0; i < entry_count; i++) {
		if (entries[i].id == id) return &entries[i];
	}
	return NULL;
}

int setup_indicator(void) {
	gtk_init(0, NULL);
	indicator = app_indicator_new("vocab", "", APP_INDICATOR_CATEGORY_APPLICATION_STATUS);
	if (!indicator) return -1;
	app_indicator_set_status(indicator, APP_INDICATOR_STATUS_ACTIVE);
	menu = gtk_menu_new();
	if (!menu) return -1;
	app_indicator_set_menu(indicator, GTK_MENU(menu));
	return 0;
}

void set_tray_icon(const char *data, int len) {
	if (temp_icon_path[0]) {
		unlink(temp_icon_path);
	}

	const char *tmpdir = getenv("TMPDIR");
	if (!tmpdir) tmpdir = "/tmp";

	strncpy(temp_icon_path, tmpdir, PATH_MAX - 20);
	strncat(temp_icon_path, "/vocab_icon_XXXXXX", PATH_MAX - 20);

	int fd = mkstemp(temp_icon_path);
	if (fd < 0) return;

	ssize_t written = write(fd, data, len);
	close(fd);

	if (written == len) {
		app_indicator_set_icon_full(indicator, temp_icon_path, "");
		app_indicator_set_attention_icon_full(indicator, temp_icon_path, "");
	}
}

static void c_menu_item_clicked(GtkWidget *widget, gpointer user_data) {
	(void)widget;
	int id = GPOINTER_TO_INT(user_data);
	goMenuItemClicked(id);
}

static gboolean c_toggle_item(GtkWidget *widget, gpointer user_data) {
	int id = GPOINTER_TO_INT(user_data);
	goMenuItemClicked(id);
	return FALSE;
}

void add_menu_item(int id, const char *label) {
	if (entry_count >= 64) return;

	GtkWidget *item = gtk_menu_item_new_with_label(label);
	g_signal_connect(G_OBJECT(item), "activate", G_CALLBACK(c_menu_item_clicked), GINT_TO_POINTER(id));
	gtk_menu_shell_append(GTK_MENU_SHELL(menu), item);
	gtk_widget_show(item);

	entries[entry_count].id = id;
	entries[entry_count].widget = item;
	entry_count++;
}

void add_check_item(int id, const char *label, int checked) {
	if (entry_count >= 64) return;

	GtkWidget *item = gtk_check_menu_item_new_with_label(label);
	gtk_check_menu_item_set_active(GTK_CHECK_MENU_ITEM(item), checked ? TRUE : FALSE);
	g_signal_connect(G_OBJECT(item), "toggled", G_CALLBACK(c_toggle_item), GINT_TO_POINTER(id));
	gtk_menu_shell_append(GTK_MENU_SHELL(menu), item);
	gtk_widget_show(item);

	entries[entry_count].id = id;
	entries[entry_count].widget = item;
	entry_count++;
}

void set_item_checked(int id, int checked) {
	MenuEntry *e = find_entry(id);
	if (e && GTK_IS_CHECK_MENU_ITEM(e->widget)) {
		gtk_check_menu_item_set_active(GTK_CHECK_MENU_ITEM(e->widget), checked ? TRUE : FALSE);
	}
}

void add_separator(void) {
	GtkWidget *sep = gtk_separator_menu_item_new();
	gtk_menu_shell_append(GTK_MENU_SHELL(menu), sep);
	gtk_widget_show(sep);
}

void remove_indicator(void) {
	if (temp_icon_path[0]) {
		unlink(temp_icon_path);
		temp_icon_path[0] = '\0';
	}
	app_indicator_set_status(indicator, APP_INDICATOR_STATUS_PASSIVE);
}
