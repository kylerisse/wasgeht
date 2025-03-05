function getDashboardData() {
    const url = '/api';
    return fetch(url)
        .then( response => response.json() )
        .then( data => data )
        .catch( error => console.log(error) );
}

const refreshInterval = 5 * 1000; // 5 seconds

function renderDashboard() {
    getDashboardData()
        .then(data => {
            const $Dashboard = document.querySelector('#dashboard-body');
            $Dashboard.innerHTML = '';
            const sortedData = Object.keys(data)
                .map(key => ({
                    name: key,
                    ...data[key]
                }));
                
            sortedData.forEach(item => { 
                const { name, address, alive, latency, lastupdate } = item;
                $Dashboard.appendChild(DashboardItem({ name, address, alive, latency, lastupdate }));
            })
        });
}

function DashboardItem(props) {
    const { name, address, alive, latency, lastupdate } = props;
    const $DashboardItem = document.createElement('div');
    $DashboardItem.classList.add('dashboard-item');
    $DashboardItem.classList.add(alive ? 'alive' : 'dead');

    $DashboardItem.innerHTML = `
        <a href="/host-detail?hostname=${name}">
            <span class="name">${name}</span>
        </a>
    `;
    return $DashboardItem;
}


renderDashboard();
const dashboardRefreshInterval = setInterval(renderDashboard, refreshInterval);
