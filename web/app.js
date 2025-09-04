async function getJSON(url) {
  const res = await fetch(url, { headers: { "Accept": "application/json" } });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

async function refreshOnce() {
  try {
    const [cpu, mem, disk, net, host, procs] = await Promise.all([
      getJSON("/api/cpu"),
      getJSON("/api/mem"),
      getJSON("/api/disk"),
      getJSON("/api/net"),
      getJSON("/api/host"),
      getJSON("/api/processes?top=15"),
    ]);
    document.querySelector("#cpu").textContent = JSON.stringify(cpu, null, 2);
    document.querySelector("#mem").textContent = JSON.stringify(mem, null, 2);
    document.querySelector("#disk").textContent = JSON.stringify(disk, null, 2);
    document.querySelector("#net").textContent = JSON.stringify(net, null, 2);
    document.querySelector("#host").textContent = JSON.stringify(host, null, 2);
    document.querySelector("#procs").textContent = JSON.stringify(procs, null, 2);
  } catch (e) {
    console.error(e);
  }
}

document.querySelector("#refresh").addEventListener("click", refreshOnce);

let timer = null;
document.querySelector("#live").addEventListener("change", (e) => {
  if (e.target.checked) {
    refreshOnce();
    timer = setInterval(refreshOnce, 2000);
  } else {
    clearInterval(timer);
    timer = null;
  }
});

refreshOnce();
