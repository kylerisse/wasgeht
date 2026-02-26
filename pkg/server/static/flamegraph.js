function getDashboardData() {
	const url = '/api';
	return fetch(url)
		.then((response) => response.json())
		.then((data) => data)
		.catch((error) => console.log(error));
}

const refreshInterval = 15 * 1000; // 15 seconds

function renderDashboard() {
	getDashboardData().then((data) => {
		const $Dashboard = document.querySelector('#dashboard-body');
		$Dashboard.innerHTML = '';
		const sortedData = Object.keys(data.hosts).map((key) => ({
			name: key,
			...data.hosts[key],
		}));

		sortedData.forEach((item) => {
			const { name, address, status } = item;
			$Dashboard.appendChild(
				DashboardItem({ name, address, status: status || 'unknown' }),
			);
		});
	});
}

function DashboardItem(props) {
	const { name, address, status } = props;
	const $DashboardItem = document.createElement('div');
	$DashboardItem.classList.add('dashboard-item');
	$DashboardItem.classList.add(`status-${status}`);

	$DashboardItem.innerHTML = `
        <a href="/host-detail?hostname=${name}">
            <span class="name">${name}</span>
        </a>
    `;
	return $DashboardItem;
}

renderDashboard();
const dashboardRefreshInterval = setInterval(renderDashboard, refreshInterval);

let countdown = 15;
function updateCountdown() {
	countdown -= 1;
	document.getElementById('countdown').textContent =
		`Next refresh in: ${countdown}s`;

	if (countdown <= 0) {
		countdown = 15;
	}
}
const countdownInterval = setInterval(updateCountdown, 1000);
