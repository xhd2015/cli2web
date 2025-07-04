document.addEventListener('DOMContentLoaded', function () {
    // Restore tree state from localStorage
    const treeState = JSON.parse(localStorage.getItem('treeState')) || {};

    document.querySelectorAll('.sidebar .caret').forEach(function(caret) {
        const path = caret.getAttribute('data-path');
        if (treeState[path]) {
            caret.classList.add('caret-down');
            const nested = caret.parentElement.querySelector(".nested");
            if (nested) {
                nested.classList.add('active');
            }
        }
    });

    // Event delegation for toggling the tree
    document.addEventListener('click', function (event) {
        if (event.target.classList.contains('caret')) {
            const path = event.target.getAttribute('data-path');
            const nested = event.target.parentElement.querySelector(".nested");
            if (nested) {
                nested.classList.toggle("active");
                event.target.classList.toggle("caret-down");
                treeState[path] = nested.classList.contains('active');
                localStorage.setItem('treeState', JSON.stringify(treeState));
            }
        }
    });

    // Expand the tree to the current page
    var path = window.location.pathname;
    var links = document.querySelectorAll('.sidebar a');
    links.forEach(function(link) {
        if (link.getAttribute('href') === path) {
            var parent = link.parentElement;
            while (parent) {
                if (parent.classList.contains('nested')) {
                    parent.classList.add('active');
                    var caret = parent.previousElementSibling;
                    if (caret && caret.classList.contains('caret')) {
                        caret.classList.add('caret-down');
                    }
                }
                parent = parent.parentElement;
            }
        }
    });

    // Handle form submission using event delegation
    document.addEventListener('submit', function (event) {
        if (event.target.id === 'command-form') {
            event.preventDefault();
            const form = event.target;
            const output = document.getElementById('output');
            const runButton = form.querySelector('button[type="submit"]');

            output.textContent = '';
            runButton.disabled = true;
            runButton.textContent = 'Running...';

            const formData = new FormData(form);
            const ws = new WebSocket('ws://' + window.location.host + '/ws' + window.location.pathname);
            console.log('WebSocket connection opened.');

            ws.onopen = () => {
                ws.send(JSON.stringify(Object.fromEntries(formData.entries())));
                console.log('Form data sent over WebSocket.');
            };

            ws.onmessage = (event) => {
                console.log('Received message from WebSocket:', event.data);
                output.textContent += event.data;
            };

            ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                runButton.disabled = false;
                runButton.textContent = 'Run';
            };

            ws.onclose = (event) => {
                console.log('WebSocket closed:', event);
                runButton.disabled = false;
                runButton.textContent = 'Run';
            };
        }
    });
});