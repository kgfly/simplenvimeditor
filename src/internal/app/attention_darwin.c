// Dock attention for macOS (editor.attention_on_bell).
//
// See openfile_darwin.c for why this is a separate translation unit rather
// than a cgo preamble.
//
// Both calls come from Go goroutines, so the AppKit work is queued onto the
// main thread asynchronously: blocking here could deadlock against a main
// thread that is itself waiting on Gio's event loop.

#import <Cocoa/Cocoa.h>

// Pending critical request; only touched on the main thread.
static NSInteger snv_attention_request = 0;

void snv_request_attention(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if ([NSApp isActive]) {
            return;
        }
        // Bounce and badge until the app is activated (switched to).
        if (snv_attention_request != 0) {
            [NSApp cancelUserAttentionRequest:snv_attention_request];
        }
        snv_attention_request = [NSApp requestUserAttention:NSCriticalRequest];
        [[NSApp dockTile] setBadgeLabel:@"\u25CF"];
    });
}

void snv_clear_attention(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (snv_attention_request != 0) {
            [NSApp cancelUserAttentionRequest:snv_attention_request];
            snv_attention_request = 0;
        }
        [[NSApp dockTile] setBadgeLabel:nil];
    });
}
