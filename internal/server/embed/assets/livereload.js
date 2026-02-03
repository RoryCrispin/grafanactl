(function() {
    'use strict';

    var script = document.currentScript;
    var port = script ? script.getAttribute('data-port') : location.port;
    var wsURL = 'ws://' + location.hostname + ':' + port + '/livereload';

    console.log('[grafanactl] livereload.js loaded, connecting to', wsURL);

    var reconnectDelay = 1000;
    var maxReconnectDelay = 30000;
    var currentDelay = reconnectDelay;

    function connect() {
        console.log('[grafanactl] connecting to WebSocket...');
        var ws = new WebSocket(wsURL);

        ws.onopen = function() {
            console.log('[grafanactl] WebSocket connected');
            currentDelay = reconnectDelay;
        };

        ws.onmessage = function(event) {
            console.log('[grafanactl] received message:', event.data);
            var msg;
            try {
                msg = JSON.parse(event.data);
            } catch (e) {
                console.error('[grafanactl] failed to parse message:', e);
                return;
            }

            if (msg.command === 'reload') {
                console.log('[grafanactl] reloading page...');
                location.reload();
            }
        };

        ws.onclose = function() {
            console.log('[grafanactl] WebSocket closed, reconnecting in', currentDelay, 'ms');
            setTimeout(function() {
                currentDelay = Math.min(currentDelay * 2, maxReconnectDelay);
                connect();
            }, currentDelay);
        };

        ws.onerror = function(err) {
            console.error('[grafanactl] WebSocket error:', err);
            ws.close();
        };
    }

    // Start connection when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', connect);
    } else {
        connect();
    }
})();
