// Dock attention for macOS (editor.attention_on_bell).
//
// See openfile_darwin.c for why this is a separate translation unit rather
// than a cgo preamble.
//
// Both calls come from Go goroutines, so the AppKit work is queued onto the
// main thread asynchronously: blocking here could deadlock against a main
// thread that is itself waiting on Gio's event loop.

#import <Cocoa/Cocoa.h>

void snv_request_attention(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if ([NSApp isActive]) {
            return;
        }
        // One bounce, plus a badge that stays until the window is focused.
        [NSApp requestUserAttention:NSInformationalRequest];
        [[NSApp dockTile] setBadgeLabel:@"\u25CF"];
    });
}

void snv_clear_attention(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSApp dockTile] setBadgeLabel:nil];
    });
}
