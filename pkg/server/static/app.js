/* wasgeht unified JS — Alpine.js CSP-safe components */

/* ── localStorage filter state (shared across pages) ──────── */

var filterState = {
    KEY_SEARCH: 'wasgeht.filters.search',
    KEY_STATUSES: 'wasgeht.filters.statuses',
    KEY_OMITTED: 'wasgeht.omitted',

    getSearch: function () {
        return localStorage.getItem(this.KEY_SEARCH) || '';
    },
    setSearch: function (val) {
        localStorage.setItem(this.KEY_SEARCH, val);
    },

    getStatuses: function () {
        try {
            var raw = localStorage.getItem(this.KEY_STATUSES);
            return raw ? JSON.parse(raw) : [];
        } catch (e) {
            return [];
        }
    },
    setStatuses: function (arr) {
        localStorage.setItem(this.KEY_STATUSES, JSON.stringify(arr));
    },

    getOmitted: function () {
        try {
            var raw = localStorage.getItem(this.KEY_OMITTED);
            return raw ? JSON.parse(raw) : [];
        } catch (e) {
            return [];
        }
    },
    setOmitted: function (arr) {
        localStorage.setItem(this.KEY_OMITTED, JSON.stringify(arr));
    }
};

/* ── Utility helpers ──────────────────────────────────────── */

var ALL_STATUSES = ['up', 'down', 'degraded', 'stale', 'pending', 'unconfigured'];

var ALL_TIMES = [
    { key: '15m', label: '15m' },
    { key: '1h',  label: '1h' },
    { key: '4h',  label: '4h' },
    { key: '8h',  label: '8h' },
    { key: '1d',  label: '1d' },
    { key: '4d',  label: '4d' },
    { key: '1w',  label: '1w' },
    { key: '31d', label: '1mo' },
    { key: '93d', label: '1q' },
    { key: '1y',  label: '1y' },
    { key: '2y',  label: '2y' },
    { key: '5y',  label: '5y' }
];

function statusLabel(status) {
    return status.replace('_', ' ');
}

function sortCompare(a, b, dir) {
    if (a < b) return dir === 'asc' ? -1 : 1;
    if (a > b) return dir === 'asc' ? 1 : -1;
    return 0;
}

var STATUS_PRIORITY = { up: 0, degraded: 1, stale: 2, pending: 3, down: 4, unconfigured: 5 };

/* ── Dashboard component ──────────────────────────────────── */

document.addEventListener('alpine:init', function () {

    Alpine.data('dashboard', function () {
        return {
            hosts: {},
            search: '',
            activeStatuses: [],
            omitted: [],
            allStatuses: ALL_STATUSES,
            sortCol: 'name',
            sortDir: 'asc',
            refreshCountdown: 5,
            _interval: null,
            _countdownInterval: null,

            init: function () {
                this.search = filterState.getSearch();
                this.activeStatuses = filterState.getStatuses();
                this.omitted = filterState.getOmitted();
                this.fetchData();
                var self = this;
                this._interval = setInterval(function () { self.fetchData(); }, 5000);
                this._countdownInterval = setInterval(function () {
                    self.refreshCountdown--;
                    if (self.refreshCountdown < 0) self.refreshCountdown = 5;
                }, 1000);
            },

            destroy: function () {
                clearInterval(this._interval);
                clearInterval(this._countdownInterval);
            },

            fetchData: function () {
                var self = this;
                self.refreshCountdown = 5;
                fetch('/api')
                    .then(function (r) { return r.json(); })
                    .then(function (data) { self.hosts = data.hosts || {}; });
            },

            filteredHosts: function () {
                var self = this;
                var entries = Object.entries(this.hosts);

                entries = entries.filter(function (entry) {
                    return self.omitted.indexOf(entry[0]) === -1;
                });

                if (this.search) {
                    var q = this.search.toLowerCase();
                    entries = entries.filter(function (entry) {
                        return entry[0].toLowerCase().indexOf(q) !== -1;
                    });
                }

                if (this.activeStatuses.length > 0) {
                    entries = entries.filter(function (entry) {
                        return self.activeStatuses.indexOf(entry[1].status) !== -1;
                    });
                }

                var col = this.sortCol;
                var dir = this.sortDir;
                entries.sort(function (a, b) {
                    if (col === 'status') {
                        var pa = STATUS_PRIORITY[a[1].status] !== undefined ? STATUS_PRIORITY[a[1].status] : 9;
                        var pb = STATUS_PRIORITY[b[1].status] !== undefined ? STATUS_PRIORITY[b[1].status] : 9;
                        return sortCompare(pa, pb, dir);
                    }
                    return sortCompare(a[0].toLowerCase(), b[0].toLowerCase(), dir);
                });

                return entries;
            },

            sortBy: function (col) {
                if (this.sortCol === col) {
                    this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
                } else {
                    this.sortCol = col;
                    this.sortDir = 'asc';
                }
            },

            sortArrow: function (col) {
                if (this.sortCol !== col) return '';
                return this.sortDir === 'asc' ? ' \u25B2' : ' \u25BC';
            },

            toggleStatus: function (status) {
                var idx = this.activeStatuses.indexOf(status);
                if (idx === -1) {
                    this.activeStatuses.push(status);
                } else {
                    this.activeStatuses.splice(idx, 1);
                }
                filterState.setStatuses(this.activeStatuses);
            },

            isStatusActive: function (status) {
                return this.activeStatuses.indexOf(status) !== -1;
            },

            updateSearch: function (val) {
                this.search = val;
                filterState.setSearch(val);
            },

            omitHostEntry: function (entry) {
                this.omitHost(entry[0]);
            },

            omitHost: function (hostname) {
                if (this.omitted.indexOf(hostname) === -1) {
                    this.omitted.push(hostname);
                    filterState.setOmitted(this.omitted);
                }
            },

            clearOmitted: function () {
                this.omitted = [];
                filterState.setOmitted([]);
            },

            checkEntries: function (checks) {
                return Object.entries(checks || {});
            },

            hostCheckEntries: function (entry) {
                return this.checkEntries(entry[1].checks);
            },

            summaryEntries: function () {
                var self = this;
                var counts = {};
                Object.entries(this.hosts).forEach(function (entry) {
                    if (self.omitted.indexOf(entry[0]) !== -1) return;
                    var s = entry[1].status;
                    counts[s] = (counts[s] || 0) + 1;
                });
                return ALL_STATUSES.filter(function (s) {
                    return (counts[s] || 0) > 0;
                }).map(function (s) {
                    return { status: s, count: counts[s] };
                });
            },

            summaryBadgeClass: function (entry) {
                return 'summary-badge status-' + entry.status;
            },

            summaryBadgeClickClass: function (entry) {
                var base = 'summary-badge summary-badge-clickable status-' + entry.status;
                if (this.activeStatuses.length > 0 && !this.isStatusActive(entry.status)) {
                    base += ' summary-badge-dimmed';
                }
                return base;
            },

            hostStatusBadgeClass: function (entry) {
                return 'summary-badge status-' + entry[1].status;
            },

            hostStatusText: function (entry) {
                return entry[1].status;
            },

            hostDetailHref: function (entry) {
                return '/host-detail?hostname=' + encodeURIComponent(entry[0]);
            },

            hostName: function (entry) {
                return entry[0];
            },

            checkBadgeClass: function (chk) {
                return 'check-badge ' + (chk[1].alive ? 'check-alive' : 'check-dead');
            },

            checkBadgeText: function (chk) {
                return chk[0] + (chk[1].alive ? ' \u2713' : ' \u2717');
            },

            statusToggleClass: function (s) {
                return 'status-toggle status-' + s + (this.isStatusActive(s) ? ' active' : '');
            },

            refreshText: function () {
                return 'Refresh in ' + this.refreshCountdown + 's';
            },

            omittedText: function () {
                return this.omitted.length + ' host(s) omitted';
            },

            hasOmitted: function () {
                return this.omitted.length > 0;
            }
        };
    });

    /* ── Grid view component ──────────────────────────────────── */

    Alpine.data('gridview', function () {
        return {
            hosts: {},
            search: '',
            activeStatuses: [],
            omitted: [],
            refreshCountdown: 5,
            _interval: null,
            _countdownInterval: null,

            init: function () {
                this.search = filterState.getSearch();
                this.activeStatuses = filterState.getStatuses();
                this.omitted = filterState.getOmitted();
                this.fetchData();
                var self = this;
                this._interval = setInterval(function () { self.fetchData(); }, 5000);
                this._countdownInterval = setInterval(function () {
                    self.refreshCountdown--;
                    if (self.refreshCountdown < 0) self.refreshCountdown = 5;
                }, 1000);
            },

            destroy: function () {
                clearInterval(this._interval);
                clearInterval(this._countdownInterval);
            },

            fetchData: function () {
                var self = this;
                self.refreshCountdown = 5;
                fetch('/api')
                    .then(function (r) { return r.json(); })
                    .then(function (data) { self.hosts = data.hosts || {}; });
            },

            filteredHosts: function () {
                var self = this;
                var entries = Object.entries(this.hosts);

                entries = entries.filter(function (entry) {
                    return self.omitted.indexOf(entry[0]) === -1;
                });

                if (this.search) {
                    var q = this.search.toLowerCase();
                    entries = entries.filter(function (entry) {
                        return entry[0].toLowerCase().indexOf(q) !== -1;
                    });
                }

                if (this.activeStatuses.length > 0) {
                    entries = entries.filter(function (entry) {
                        return self.activeStatuses.indexOf(entry[1].status) !== -1;
                    });
                }

                entries.sort(function (a, b) {
                    return sortCompare(a[0].toLowerCase(), b[0].toLowerCase(), 'asc');
                });

                return entries;
            },

            toggleStatus: function (status) {
                var idx = this.activeStatuses.indexOf(status);
                if (idx === -1) {
                    this.activeStatuses.push(status);
                } else {
                    this.activeStatuses.splice(idx, 1);
                }
                filterState.setStatuses(this.activeStatuses);
            },

            isStatusActive: function (status) {
                return this.activeStatuses.indexOf(status) !== -1;
            },

            updateSearch: function (val) {
                this.search = val;
                filterState.setSearch(val);
            }
        };
    });

    /* ── Host detail component ────────────────────────────────── */

    Alpine.data('hostdetail', function () {
        return {
            hostname: '',
            host: null,
            checkTypes: [],
            visibleChecks: [],
            visibleTimes: [],
            allTimes: ALL_TIMES,
            loading: true,

            init: function () {
                var params = new URLSearchParams(window.location.search);
                this.hostname = params.get('hostname') || '';
                this.visibleTimes = ALL_TIMES.map(function (t) { return t.key; });
                if (this.hostname) {
                    this.fetchHost();
                }
            },

            fetchHost: function () {
                var self = this;
                fetch('/api/hosts/' + encodeURIComponent(this.hostname))
                    .then(function (r) { return r.json(); })
                    .then(function (data) {
                        self.host = data;
                        self.checkTypes = Object.keys(data.checks || {}).sort();
                        self.visibleChecks = self.checkTypes.slice();
                        self.loading = false;
                    })
                    .catch(function () {
                        self.loading = false;
                    });
            },

            toggleCheck: function (checkType) {
                var idx = this.visibleChecks.indexOf(checkType);
                if (idx === -1) {
                    this.visibleChecks.push(checkType);
                } else {
                    this.visibleChecks.splice(idx, 1);
                }
            },

            isCheckVisible: function (checkType) {
                return this.visibleChecks.indexOf(checkType) !== -1;
            },

            toggleTime: function (timeKey) {
                var idx = this.visibleTimes.indexOf(timeKey);
                if (idx === -1) {
                    this.visibleTimes.push(timeKey);
                } else {
                    this.visibleTimes.splice(idx, 1);
                }
            },

            isTimeVisible: function (timeKey) {
                return this.visibleTimes.indexOf(timeKey) !== -1;
            },

            showAllChecks: function () {
                this.visibleChecks = this.checkTypes.slice();
            },

            showAllTimes: function () {
                this.visibleTimes = ALL_TIMES.map(function (t) { return t.key; });
            },

            imgSrc: function (checkType, timeKey) {
                return '/imgs/' + this.hostname + '/' + this.hostname + '_' + checkType + '_' + timeKey + '.png?t=' + Date.now();
            },

            checkLabel: function (checkType) {
                return checkType.replace('_', ' ');
            }
        };
    });

});
