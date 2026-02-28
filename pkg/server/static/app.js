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

function checkSummaryMetric(checkType, data) {
    if (!data || !data.alive || !data.metrics) return '';
    var metrics = data.metrics;
    if (checkType === 'wifi_stations') {
        var total = metrics['total'];
        return total !== undefined ? ' ' + total : '';
    }
    var vals = Object.values(metrics);
    if (vals.length === 0) return '';
    var avg = vals.reduce(function (a, b) { return a + b; }, 0) / vals.length;
    return ' ' + (avg / 1000).toFixed(1) + 'ms';
}

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

            clearStatuses: function () {
                this.activeStatuses = [];
                filterState.setStatuses([]);
            },

            hasActiveStatuses: function () {
                return this.activeStatuses.length > 0;
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

            restoreHost: function (hostname) {
                var idx = this.omitted.indexOf(hostname);
                if (idx !== -1) {
                    this.omitted.splice(idx, 1);
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
                var symbol = chk[1].alive ? ' \u2713' : ' \u2717';
                var metric = checkSummaryMetric(chk[0], chk[1]);
                return chk[0] + symbol + metric;
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
            allStatuses: ALL_STATUSES,
            refreshCountdown: 5,
            winW: window.innerWidth,
            winH: window.innerHeight,
            _interval: null,
            _countdownInterval: null,
            _resizeHandler: null,

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
                this._resizeHandler = function () {
                    self.winW = window.innerWidth;
                    self.winH = window.innerHeight;
                };
                window.addEventListener('resize', this._resizeHandler);
            },

            destroy: function () {
                clearInterval(this._interval);
                clearInterval(this._countdownInterval);
                window.removeEventListener('resize', this._resizeHandler);
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

            clearStatuses: function () {
                this.activeStatuses = [];
                filterState.setStatuses([]);
            },

            hasActiveStatuses: function () {
                return this.activeStatuses.length > 0;
            },

            updateSearch: function (val) {
                this.search = val;
                filterState.setSearch(val);
            },

            omitHostEntry: function (entry) {
                var hostname = entry[0];
                if (this.omitted.indexOf(hostname) === -1) {
                    this.omitted.push(hostname);
                    filterState.setOmitted(this.omitted);
                }
            },

            restoreHost: function (hostname) {
                var idx = this.omitted.indexOf(hostname);
                if (idx !== -1) {
                    this.omitted.splice(idx, 1);
                    filterState.setOmitted(this.omitted);
                }
            },

            clearOmitted: function () {
                this.omitted = [];
                filterState.setOmitted([]);
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

            summaryBadgeClickClass: function (entry) {
                var base = 'summary-badge summary-badge-clickable status-' + entry.status;
                if (this.activeStatuses.length > 0 && !this.isStatusActive(entry.status)) {
                    base += ' summary-badge-dimmed';
                }
                return base;
            },

            gridItemClass: function (entry) {
                return 'grid-item status-' + entry[1].status;
            },

            gridStyle: function () {
                var n = this.filteredHosts().length;
                if (n === 0) return {};
                var grid = document.querySelector('.grid-view');
                var gridTop = grid ? grid.getBoundingClientRect().top : 60;
                var availH = this.winH - gridTop;
                var availW = this.winW - 32;
                var ratio = availW / availH;
                var cols = Math.max(1, Math.round(Math.sqrt(n * ratio)));
                var rows = Math.ceil(n / cols);
                return {
                    gridTemplateColumns: 'repeat(' + cols + ', 1fr)',
                    gridTemplateRows: 'repeat(' + rows + ', 1fr)',
                    height: availH + 'px',
                };
            },

            hostDetailHref: function (entry) {
                return '/host-detail?hostname=' + encodeURIComponent(entry[0]);
            },

            hostName: function (entry) {
                return entry[0];
            },

            omittedText: function () {
                return this.omitted.length + ' host(s) omitted';
            },

            hasOmitted: function () {
                return this.omitted.length > 0;
            }
        };
    });

    /* ── Host detail component ────────────────────────────────── */

    Alpine.data('hostdetail', function () {
        return {
            hostname: '',
            host: null,
            allHosts: {},
            omitted: [],
            checkTypes: [],
            visibleChecks: [],
            allTimes: ALL_TIMES,
            loading: true,
            graphTimestamp: Date.now(),
            modalSrc: '',
            modalAlt: '',
            modalOpen: false,
            _statusInterval: null,
            _graphInterval: null,

            init: function () {
                var self = this;
                var params = new URLSearchParams(window.location.search);
                this.hostname = params.get('hostname') || '';
                this.omitted = filterState.getOmitted();
                if (this.hostname) {
                    this.fetchHost();
                    this.fetchAllHosts();
                    this._statusInterval = setInterval(function () {
                        self.fetchHost();
                        self.fetchAllHosts();
                    }, 5000);
                    this._graphInterval = setInterval(function () {
                        self.graphTimestamp = Date.now();
                    }, 60000);
                }
            },

            destroy: function () {
                clearInterval(this._statusInterval);
                clearInterval(this._graphInterval);
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

            fetchAllHosts: function () {
                var self = this;
                fetch('/api')
                    .then(function (r) { return r.json(); })
                    .then(function (data) { self.allHosts = data.hosts || {}; });
            },

            summaryEntries: function () {
                var self = this;
                var counts = {};
                Object.entries(this.allHosts).forEach(function (entry) {
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

            hasOmitted: function () {
                return this.omitted.length > 0;
            },

            omittedText: function () {
                return this.omitted.length + ' host(s) omitted';
            },

            restoreHost: function (hostname) {
                var idx = this.omitted.indexOf(hostname);
                if (idx !== -1) {
                    this.omitted.splice(idx, 1);
                    filterState.setOmitted(this.omitted);
                }
            },

            clearOmitted: function () {
                this.omitted = [];
                filterState.setOmitted([]);
            },

            openModal: function (checkType, t) {
                this.modalSrc = this.imgSrc(checkType, t.key);
                this.modalAlt = this.hostname + ' ' + checkType + ' ' + t.label;
                this.modalOpen = true;
            },

            closeModal: function () {
                this.modalOpen = false;
                this.modalSrc = '';
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

            showAllChecks: function () {
                this.visibleChecks = this.checkTypes.slice();
            },

            imgSrc: function (checkType, timeKey) {
                return '/imgs/' + this.hostname + '/' + this.hostname + '_' + checkType + '_' + timeKey + '.png?t=' + this.graphTimestamp;
            },

            checkLabel: function (checkType) {
                return checkType.replace('_', ' ');
            },

            checkAlive: function (checkType) {
                return this.host && this.host.checks && this.host.checks[checkType] && this.host.checks[checkType].alive;
            },

            checkToggleClass: function (checkType) {
                var alive = this.checkAlive(checkType);
                var cls = 'check-filter-btn ' + (alive ? 'check-alive' : 'check-dead');
                if (!this.isCheckVisible(checkType)) cls += ' check-filter-dimmed';
                return cls;
            },

            checkToggleText: function (checkType) {
                var alive = this.checkAlive(checkType);
                var symbol = alive ? ' \u2713' : ' \u2717';
                var metric = checkSummaryMetric(checkType, this.host && this.host.checks && this.host.checks[checkType]);
                return this.checkLabel(checkType) + symbol + metric;
            },

            hostStatusBadgeClass: function () {
                if (!this.host) return 'summary-badge';
                return 'summary-badge status-' + this.host.status;
            },

            statusText: function () {
                if (!this.host) return '';
                return this.host.status;
            },

            hostTags: function () {
                if (!this.host || !this.host.tags) return [];
                return Object.entries(this.host.tags);
            },

            tagText: function (tag) {
                return tag[0] + ': ' + tag[1];
            },

            visibleCheckTypes: function () {
                var self = this;
                return this.checkTypes.filter(function (ct) {
                    return self.isCheckVisible(ct);
                });
            },

            graphImgSrc: function (checkType, t) {
                return this.imgSrc(checkType, t.key);
            },

            graphAlt: function (checkType, t) {
                return this.hostname + ' ' + checkType + ' ' + t.label;
            }
        };
    });

});
