//go:build linux

// Attention for Wayland windows (editor.attention_on_bell), via
// xdg-activation-v1.
//
// Activating a surface with a token that carries no input serial cannot
// steal focus, so compositors mark the window as demanding attention
// instead: GNOME shows a "... is ready" notification, KDE highlights the
// taskbar entry. SDL and GLFW request attention the same way.
//
// Like drop_wayland.c, this runs on Gio's wl_display from another thread,
// so every proxy lives on a private wl_event_queue.
//
// The protocol's interface tables are written out by hand, which is all
// wayland-scanner would generate for this small protocol.

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <wayland-client.h>

extern const struct wl_interface snv_xdg_activation_token_v1_interface;

static const struct wl_interface *snv_xa_types[] = {
    NULL,
    &snv_xdg_activation_token_v1_interface, // get_activation_token: n
    NULL, &wl_surface_interface,            // activate: s o
    NULL, &wl_seat_interface,               // set_serial: u o
    &wl_surface_interface,                  // set_surface: o
};

enum {
    SNV_XA_DESTROY = 0,
    SNV_XA_GET_ACTIVATION_TOKEN = 1,
    SNV_XA_ACTIVATE = 2,
};

static const struct wl_message snv_xa_requests[] = {
    {"destroy", "", snv_xa_types},
    {"get_activation_token", "n", snv_xa_types + 1},
    {"activate", "so", snv_xa_types + 2},
};

const struct wl_interface snv_xdg_activation_v1_interface = {
    "xdg_activation_v1", 1, 3, snv_xa_requests, 0, NULL,
};

enum {
    SNV_XAT_SET_SURFACE = 2,
    SNV_XAT_COMMIT = 3,
    SNV_XAT_DESTROY = 4,
};

static const struct wl_message snv_xat_requests[] = {
    {"set_serial", "uo", snv_xa_types + 4},
    {"set_app_id", "s", snv_xa_types},
    {"set_surface", "o", snv_xa_types + 6},
    {"commit", "", snv_xa_types},
    {"destroy", "", snv_xa_types},
};

static const struct wl_message snv_xat_events[] = {
    {"done", "s", snv_xa_types},
};

const struct wl_interface snv_xdg_activation_token_v1_interface = {
    "xdg_activation_token_v1", 1, 5, snv_xat_requests, 1, snv_xat_events,
};

struct snv_xa {
    struct wl_proxy *activation;
    char *token;
};

static void snv_xa_global(void *data, struct wl_registry *registry,
                          uint32_t name, const char *iface,
                          uint32_t version) {
    struct snv_xa *s = data;
    (void)version;
    if (s->activation == NULL && strcmp(iface, "xdg_activation_v1") == 0) {
        s->activation = wl_registry_bind(registry, name,
                                         &snv_xdg_activation_v1_interface, 1);
    }
}

static void snv_xa_global_remove(void *data, struct wl_registry *registry,
                                 uint32_t name) {
    (void)data;
    (void)registry;
    (void)name;
}

static const struct wl_registry_listener snv_xa_registry_listener = {
    snv_xa_global,
    snv_xa_global_remove,
};

static void snv_xa_token_done(void *data, struct wl_proxy *token,
                              const char *value) {
    struct snv_xa *s = data;
    (void)token;
    if (s->token == NULL) {
        s->token = strdup(value);
    }
}

static const struct {
    void (*done)(void *, struct wl_proxy *, const char *);
} snv_xa_token_listener = {snv_xa_token_done};

void snv_wl_request_attention(void *display, void *surface) {
    struct wl_display *dpy = display;
    struct snv_xa s;
    memset(&s, 0, sizeof(s));

    struct wl_event_queue *queue = wl_display_create_queue(dpy);
    if (queue == NULL) {
        return;
    }
    // A wrapper puts the registry on our queue from the moment it exists,
    // so none of its events can be dispatched by Gio's thread.
    struct wl_display *wrapper = wl_proxy_create_wrapper(dpy);
    if (wrapper == NULL) {
        wl_event_queue_destroy(queue);
        return;
    }
    wl_proxy_set_queue((struct wl_proxy *)wrapper, queue);
    struct wl_registry *registry = wl_display_get_registry(wrapper);
    wl_proxy_wrapper_destroy(wrapper);
    if (registry == NULL) {
        wl_event_queue_destroy(queue);
        return;
    }
    wl_registry_add_listener(registry, &snv_xa_registry_listener, &s);

    if (wl_display_roundtrip_queue(dpy, queue) >= 0 && s.activation != NULL) {
        struct wl_proxy *token = wl_proxy_marshal_constructor(
            s.activation, SNV_XA_GET_ACTIVATION_TOKEN,
            &snv_xdg_activation_token_v1_interface, NULL);
        if (token != NULL) {
            wl_proxy_add_listener(token, (void (**)(void))&snv_xa_token_listener, &s);
            wl_proxy_marshal(token, SNV_XAT_SET_SURFACE, surface);
            wl_proxy_marshal(token, SNV_XAT_COMMIT);
            for (int i = 0; i < 3 && s.token == NULL; i++) {
                if (wl_display_roundtrip_queue(dpy, queue) < 0) {
                    break;
                }
            }
            if (s.token != NULL) {
                wl_proxy_marshal(s.activation, SNV_XA_ACTIVATE, s.token, surface);
                free(s.token);
            }
            wl_proxy_marshal(token, SNV_XAT_DESTROY);
            wl_proxy_destroy(token);
        }
        wl_proxy_marshal(s.activation, SNV_XA_DESTROY);
        wl_proxy_destroy(s.activation);
        wl_display_flush(dpy);
    }
    wl_registry_destroy(registry);
    wl_event_queue_destroy(queue);
}
