let sortOrder = [true, true]; // Keeps track of sorting order for each column (true = ascending, false = descending)
let sortedColumn = -1; // Keeps track of which column is currently sorted (-1 means no column is sorted)
let countdown = 5; // Refresh countdown in seconds

// Throttle function to limit the rate at which a function can fire.
function throttle(func, limit) {
    let inThrottle;
    return function() {
        const args = arguments;
        const context = this;
        if (!inThrottle) {
            func.apply(context, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    }
}

document.addEventListener("DOMContentLoaded", function () {
    // Initial load of the table data
    loadTableData();

    // Set interval to refresh table data every 5 seconds
    setInterval(loadTableData, 5000);

    // Set interval to update countdown every second
    setInterval(updateCountdown, 1000);
});

function loadTableData() {
    fetch("/api")
        .then(response => response.json())
        .then(data => {
            updateTable(data);
            countdown = 5; // Reset countdown after loading new data

            // Apply sorting again if a column is already sorted
            if (sortedColumn !== -1) {
                sortTable(sortedColumn, false);
            }
        })
        .catch(error => console.error('Error fetching host data:', error));
}

function updateTable(data) {
    // Remove all existing graph popups to prevent lingering images
    document.querySelectorAll('.graph-popup').forEach(el => el.remove());

    const tbody = document.getElementById("hostTable").getElementsByTagName("tbody")[0];
    tbody.innerHTML = ''; // Clear existing rows

    for (const [host, info] of Object.entries(data)) {
        const row = tbody.insertRow();
        row.className = info.alive ? "up" : "down";

        const cellHost = row.insertCell(0);
        cellHost.textContent = host;

        const cellStatus = row.insertCell(1);
        cellStatus.textContent = info.alive ? "UP" : "DOWN";

        // Create the graph popup but don't append it to the cell
        const graphContainer = document.createElement("div");
        graphContainer.className = "graph-popup";

        const img = document.createElement("img");
        // Append a timestamp to prevent caching
        img.src = `/imgs/${host}/${host}_latency_1h.png?${new Date().getTime()}`;
        img.alt = `Latency graph for ${host}`;
        img.width = 600; // Adjust dimensions as needed
        img.height = 200;

        graphContainer.appendChild(img);
        document.body.appendChild(graphContainer); // Append to body

        // Add event listeners to cellStatus
        cellStatus.addEventListener('mouseenter', function (e) {
            // Update the image when the mouse enters
            updateImage();
            graphContainer.style.display = 'block';
        });

        // Preload image when updating
        function updateImage() {
            const timestamp = new Date().getTime();
            const newSrc = `/imgs/${host}/${host}_latency_1h.png?${timestamp}`;
            const preloadImg = new Image();
            preloadImg.src = newSrc;
            preloadImg.onload = () => {
                img.src = newSrc;
            };
        }

        cellStatus.addEventListener('mousemove', throttle(function (e) {
            // Update the image to fetch the latest graph
            updateImage();

            // Get mouse position
            let mouseX = e.clientX;
            let mouseY = e.clientY;

            // Adjust position to place the graph to the right of the mouse pointer
            let graphWidth = img.width;
            let graphHeight = img.height;

            // Calculate position, adjust to keep within viewport
            let xOffset = 20; // Offset to the right of the mouse pointer
            let yOffset = -20; // Offset above the mouse pointer

            let left = mouseX + xOffset;
            let top = mouseY + yOffset;

            // Get viewport dimensions
            let viewportWidth = window.innerWidth;
            let viewportHeight = window.innerHeight;

            // Adjust left position if the graph goes off the right edge
            if ((left + graphWidth) > viewportWidth) {
                left = mouseX - graphWidth - xOffset;
            }

            // Adjust top position if the graph goes off the bottom edge
            if ((top + graphHeight) > viewportHeight) {
                top = viewportHeight - graphHeight - 10; // 10px padding from bottom
            }

            // Adjust top position if the graph goes off the top edge
            if (top < 0) {
                top = 10; // 10px padding from top
            }

            graphContainer.style.left = `${left + window.scrollX}px`;
            graphContainer.style.top = `${top + window.scrollY}px`;
        }, 2000)); // Throttle to execute at most once every 2 seconds

        cellStatus.addEventListener('mouseleave', function () {
            graphContainer.style.display = 'none';
        });
    }
}

function updateCountdown() {
    countdown -= 1;
    document.getElementById("countdown").textContent = `Next refresh in: ${countdown}s`;

    if (countdown <= 0) {
        countdown = 5;
    }
}

function handleSort(columnIndex) {
    // Toggle the sort order for the column
    sortOrder[columnIndex] = !sortOrder[columnIndex];
    // Set the sorted column index
    sortedColumn = columnIndex;
    // Sort the table
    sortTable(columnIndex);
}

function sortTable(columnIndex, toggleSortOrder = true) {
    const table = document.getElementById("hostTable");
    const rows = Array.from(table.rows).slice(1); // Skip header row
    const isAscending = sortOrder[columnIndex];

    const sortedRows = rows.sort((a, b) => {
        const aText = a.cells[columnIndex].textContent;
        const bText = b.cells[columnIndex].textContent;

        if (columnIndex === 1) { // Sort by status: UP before DOWN
            if (aText === bText) return 0;
            return (aText === "UP" ? -1 : 1) * (isAscending ? 1 : -1);
        } else { // Sort by host name
            return aText.localeCompare(bText) * (isAscending ? 1 : -1);
        }
    });

    // Rebuild table body
    const tbody = table.getElementsByTagName("tbody")[0];
    tbody.innerHTML = '';
    sortedRows.forEach(row => tbody.appendChild(row));

    // Track the sorted column index
    sortedColumn = columnIndex;

    // Update headers to reflect sorting direction
    updateHeaders();
}

function updateHeaders() {
    const headers = document.querySelectorAll("#hostTable th");

    headers.forEach((header) => {
        // Simply clear any arrows that might exist
        header.textContent = header.textContent.replace(/[\u25B2\u25BC]/g, '');
    });
}
