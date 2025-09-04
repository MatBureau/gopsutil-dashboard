// ---------- Helpers ----------
const $ = (q, root = document) => root.querySelector(q);
const el = (tag, attrs = {}, ...children) => {
    const n = document.createElement(tag);
    Object.entries(attrs).forEach(([k, v]) => (k in n) ? n[k] = v : n.setAttribute(k, v));
    children.flat().forEach(c => n.append(c.nodeType ? c : document.createTextNode(c)));
    return n;
};
const fmtPct = v => `${(v ?? 0).toFixed(1)}%`;
const fmtBytes = v => {
    const u = ["B", "KB", "MB", "GB", "TB", "PB"]; let i = 0; let x = Number(v || 0);
    while (x >= 1024 && i < u.length - 1) { x /= 1024; i++; }
    return `${x.toFixed(1)} ${u[i]}`;
};
const fmtRate = v => `${fmtBytes(v)}/s`;
const fmtSecs = s => {
    s = Math.max(0, Number(s || 0)); const d = Math.floor(s / 86400);
    const h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
    return [d ? `${d}j` : null, h ? `${h}h` : null, m ? `${m}m` : `${Math.floor(s % 60)}s`].filter(Boolean).join(" ");
};

// ---------- State ----------
let cpuChart, memChart, netChart;
const cpuSeries = [];           // last N avg values
const netSeries = { rx: [], tx: [] };
const NET_WINDOW = 60;          // samples kept
const CPU_WINDOW = 60;

let prevNetTotals = null;       // for throughput delta
let prevNetByIf = new Map();    // per-interface delta for table

let processesCache = [];        // latest Top from API

// ---------- Charts ----------
function ensureCharts() {
    if (!window.Chart) return; // graceful fallback if CDN blocked

    if (!cpuChart) {
        cpuChart = new Chart($("#cpuLine"), {
            type: "line",
            data: { labels: [], datasets: [{ label: "CPU moyen (%)", data: [] }] },
            options: { responsive: true, tension: .25, scales: { y: { min: 0, max: 100 } } }
        });
    }
    if (!memChart) {
        memChart = new Chart($("#memDoughnut"), {
            type: "doughnut",
            data: { labels: ["Utilisée", "Libre"], datasets: [{ data: [0, 100] }] },
            options: { cutout: "65%" }
        });
    }
    if (!netChart) {
        netChart = new Chart($("#netLine"), {
            type: "line",
            data: {
                labels: [], datasets: [
                    { label: "↓ RX", data: [] },
                    { label: "↑ TX", data: [] },
                ]
            },
            options: { responsive: true, tension: .25, scales: { y: { beginAtZero: true } } }
        });
    }
}

// ---------- Rendering ----------
function renderCPU(cpu) {
    const per = cpu?.percent || [];
    const avg = per.reduce((a, b) => a + b, 0) / Math.max(1, per.length);

    // header pill
    $("#cpu-avg").textContent = fmtPct(avg);

    // chart
    ensureCharts();
    const ts = new Date().toLocaleTimeString();
    cpuSeries.push({ t: ts, v: +avg.toFixed(2) });
    if (cpuSeries.length > CPU_WINDOW) cpuSeries.shift();
    if (cpuChart) {
        cpuChart.data.labels = cpuSeries.map(p => p.t);
        cpuChart.data.datasets[0].data = cpuSeries.map(p => p.v);
        cpuChart.update("none");
    }

    // per-core bars
    const wrap = $("#cpu-cores");
    wrap.innerHTML = "";
    per.forEach((p, i) => {
        const bar = el("div", { className: "bar" }, el("span", { style: `width:${p}%` }), el("b", {}, `CPU${i}: ${fmtPct(p)}`));
        wrap.append(el("div", {}, bar));
    });
}

function renderMem(mem) {
    const vm = mem?.virtual;
    if (!vm) return;
    const used = vm.used;
    const free = vm.total - used;

    $("#mem-usage").textContent = `${fmtBytes(used)} / ${fmtBytes(vm.total)} (${fmtPct(vm.usedPercent)})`;
    $("#mem-total").textContent = fmtBytes(vm.total);
    $("#mem-used").textContent = `${fmtBytes(used)} (${fmtPct(vm.usedPercent)})`;
    $("#mem-free").textContent = fmtBytes(free);
    $("#mem-swap").textContent = mem.swap ? `${fmtBytes(mem.swap.used)} / ${fmtBytes(mem.swap.total)}` : "—";

    ensureCharts();
    if (memChart) {
        memChart.data.datasets[0].data = [used, free];
        memChart.update("none");
    }
}

function renderDisk(disk) {
    const usage = disk?.usage || {};
    const parts = disk?.partitions || [];
    const filter = ($("#diskFilter").value || "").toLowerCase();

    // Map mount -> usage
    const rows = parts.map(p => {
        const u = usage[p.mountpoint];
        const used = u ? u.used : 0;
        const total = u ? u.total : 0;
        const pct = total ? (used / total * 100) : 0;
        return {
            mount: p.mountpoint,
            fstype: p.fstype,
            device: p.device,
            total, used, pct
        };
    }).filter(r => !filter || [r.mount, r.fstype, r.device].join(" ").toLowerCase().includes(filter));

    const tbl = el("table", {},
        el("thead", {}, el("tr", {},
            el("th", {}, "Point de montage"),
            el("th", {}, "Type"),
            el("th", {}, "Taille"),
            el("th", {}, "Utilisation"),
        )),
        el("tbody", {},
            rows.map(r => el("tr", {},
                el("td", {}, r.mount),
                el("td", {}, r.fstype || "—"),
                el("td", {}, fmtBytes(r.total)),
                el("td", {},
                    el("div", { className: "bar" }, el("span", { style: `width:${r.pct.toFixed(1)}%` }), el("b", {}, fmtPct(r.pct)))
                ),
            ))
        )
    );
    $("#diskTable").innerHTML = "";
    $("#diskTable").append(tbl);
}

function computeNetRates(ioCounters) {
    // Sum over all interfaces
    const sum = ioCounters.reduce((a, n) => ({ rx: a.rx + n.bytesRecv, tx: a.tx + n.bytesSent }), { rx: 0, tx: 0 });
    const now = Date.now();
    let rxRate = 0, txRate = 0;
    if (prevNetTotals) {
        const dt = Math.max(1, (now - prevNetTotals.ts) / 1000); // sec
        rxRate = (sum.rx - prevNetTotals.rx) / dt;
        txRate = (sum.tx - prevNetTotals.tx) / dt;
    }
    prevNetTotals = { ...sum, ts: now };
    return { rxRate, txRate };
}

function computePerIfRates(ioCounters) {
    const now = Date.now();
    const rows = ioCounters.map(n => {
        const prev = prevNetByIf.get(n.name);
        let rx = 0, tx = 0;
        if (prev) {
            const dt = Math.max(1, (now - prev.ts) / 1000);
            rx = (n.bytesRecv - prev.rx) / dt;
            tx = (n.bytesSent - prev.tx) / dt;
        }
        prevNetByIf.set(n.name, { rx: n.bytesRecv, tx: n.bytesSent, ts: now });
        return { name: n.name, rx, tx, totRx: n.bytesRecv, totTx: n.bytesSent };
    });
    return rows;
}

function renderNet(net) {
    const ios = net?.io_counters || [];
    if (!ios.length) return;

    const { rxRate, txRate } = computeNetRates(ios);
    $("#net-rate").textContent = `↓ ${fmtRate(rxRate)} / ↑ ${fmtRate(txRate)}`;

    ensureCharts();
    const ts = new Date().toLocaleTimeString();
    netSeries.rx.push({ t: ts, v: +rxRate.toFixed(0) });
    netSeries.tx.push({ t: ts, v: +txRate.toFixed(0) });
    if (netSeries.rx.length > NET_WINDOW) { netSeries.rx.shift(); netSeries.tx.shift(); }
    if (netChart) {
        netChart.data.labels = netSeries.rx.map(p => p.t);
        netChart.data.datasets[0].data = netSeries.rx.map(p => p.v);
        netChart.data.datasets[1].data = netSeries.tx.map(p => p.v);
        netChart.update("none");
    }

    // Per-interface table
    const perIf = computePerIfRates(ios);
    perIf.sort((a, b) => (b.rx + b.tx) - (a.rx + a.tx));
    const tbl = el("table", {},
        el("thead", {}, el("tr", {},
            el("th", {}, "Interface"),
            el("th", {}, "↓ RX"),
            el("th", {}, "↑ TX"),
            el("th", {}, "Total RX"),
            el("th", {}, "Total TX"),
        )),
        el("tbody", {},
            perIf.map(r => el("tr", {},
                el("td", {}, r.name),
                el("td", {}, fmtRate(r.rx)),
                el("td", {}, fmtRate(r.tx)),
                el("td", {}, fmtBytes(r.totRx)),
                el("td", {}, fmtBytes(r.totTx)),
            ))
        )
    );
    $("#netTable").innerHTML = "";
    $("#netTable").append(tbl);
}

function renderHost(host) {
    const i = host?.info;
    if (!i) return;
    $("#host-name").textContent = i.hostname || "—";
    $("#host-os").textContent = `${i.platform || i.os} ${i.platformVersion || ""}`.trim();
    $("#host-uptime").textContent = fmtSecs(i.uptime);
    $("#host-kernel").textContent = i.kernelVersion || "—";
}

function renderProcs(list) {
    processesCache = Array.isArray(list) ? list : [];

    const q = ($("#procFilter").value || "").toLowerCase();
    const topN = parseInt($("#procTopN").value, 10) || 15;

    let rows = processesCache;
    if (q) {
        rows = rows.filter(p =>
            (p.name || "").toLowerCase().includes(q) ||
            (p.username || "").toLowerCase().includes(q) ||
            (p.cmdline || "").toLowerCase().includes(q)
        );
    }
    rows = rows.slice(0, topN);

    const tbl = el("table", {},
        el("thead", {}, el("tr", {},
            el("th", {}, "PID"),
            el("th", {}, "Nom"),
            el("th", {}, "CPU %"),
            el("th", {}, "RAM %"),
            el("th", {}, "Utilisateur"),
            el("th", {}, "Statut"),
        )),
        el("tbody", {},
            rows.map(p => el("tr", {},
                el("td", {}, p.pid),
                el("td", {}, p.name || "—"),
                el("td", {}, (p.cpu_percent ?? 0).toFixed(1)),
                el("td", {}, (p.mem_percent ?? 0).toFixed(1)),
                el("td", {}, p.username || "—"),
                el("td", {}, p.status || "—"),
            ))
        )
    );
    $("#procTable").innerHTML = "";
    $("#procTable").append(tbl);
}

// ---------- Data fetch ----------
async function getAll() {
    const res = await fetch("/api/all", { headers: { "Accept": "application/json" } });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
}

async function getHash() {
  const res = await fetch("/api/hash", { headers: { "Accept": "application/json" } });
  if (!res.ok) { console.error(await res.text()); return; }
  const d = await res.json();
  document.querySelector("#hash-val").textContent = d.randhash || "—";
  document.querySelector("#hash-bytes").textContent = d.hashbytes_hex || "—";
  document.querySelector("#hash-ts").textContent = d.updatedAt || "—";
}
document.querySelector("#getHash").addEventListener("click", getHash);


async function refreshOnce() {
    try {
        const data = await getAll();
        renderCPU(data.cpu);
        renderMem(data.mem);
        renderDisk(data.disk);
        renderNet(data.net);
        renderHost(data.host);
        renderProcs(data.processes?.top);
    } catch (e) {
        console.error(e);
    }
}

// ---------- Wiring ----------
$("#refresh").addEventListener("click", refreshOnce);
$("#live").addEventListener("change", (e) => {
    if (e.target.checked) {
        refreshOnce();
        window.__timer = setInterval(refreshOnce, 2000);
    } else {
        clearInterval(window.__timer); window.__timer = null;
    }
});
$("#diskFilter").addEventListener("input", () => renderDisk(window.__lastDisk || {}));
$("#procFilter").addEventListener("input", () => renderProcs(processesCache));
$("#procTopN").addEventListener("change", () => renderProcs(processesCache));

// Keep last disk for local filter re-render
const _origRenderDisk = renderDisk;
renderDisk = function (d) { window.__lastDisk = d; _origRenderDisk(d); };

// Init
ensureCharts();
refreshOnce();
