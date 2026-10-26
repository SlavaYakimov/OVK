const $ = (id) => document.getElementById(id);
const FIELDS = ["full_name", "phone", "room", "check_in", "check_out", "note"];
let token = localStorage.getItem("token");

async function api(method, path, body) {
  const res = await fetch("/api" + path, {
    method,
    headers: { "Content-Type": "application/json", ...(token && { Authorization: "Bearer " + token }) },
    body: body && JSON.stringify(body),
  });
  if (res.status === 401 && token) { logout(); throw new Error("Сессия истекла, войдите снова"); }
  if (res.status === 204) return null;
  const data = await res.json();
  if (!res.ok) {
    const fields = data.fields ? ": " + Object.entries(data.fields).map(([k, v]) => `${k} — ${v}`).join("; ") : "";
    throw new Error(data.error + fields);
  }
  return data;
}

function show(err) { $("msg").textContent = err ? err.message || err : ""; }
function render() {
  $("auth").classList.toggle("hidden", !!token);
  $("main").classList.toggle("hidden", !token);
  if (token) loadGuests();
}
const creds = () => ({ email: $("email").value, password: $("password").value });

async function login() {
  try { token = (await api("POST", "/login", creds())).token; localStorage.setItem("token", token); show(); render(); }
  catch (e) { show(e); }
}
async function register() {
  try { await api("POST", "/register", creds()); await login(); } catch (e) { show(e); }
}
function logout() { token = null; localStorage.removeItem("token"); render(); }

async function loadGuests() {
  try {
    const guests = await api("GET", "/guests?q=" + encodeURIComponent($("q").value));
    $("rows").innerHTML = "";
    for (const g of guests) {
      const tr = document.createElement("tr");
      for (const f of FIELDS) { const td = document.createElement("td"); td.textContent = g[f]; tr.append(td); }
      const td = document.createElement("td");
      const edit = document.createElement("button"); edit.textContent = "✎"; edit.onclick = () => fillForm(g);
      const del = document.createElement("button"); del.textContent = "✕"; del.onclick = () => deleteGuest(g.id);
      td.append(edit, del); tr.append(td); $("rows").append(tr);
    }
    show();
  } catch (e) { show(e); }
}

function fillForm(g) { $("id").value = g.id; FIELDS.forEach((f) => ($(f).value = g[f])); }
function resetForm() { $("form").reset(); $("id").value = ""; }

async function saveGuest(ev) {
  ev.preventDefault();
  const g = Object.fromEntries(FIELDS.map((f) => [f, $(f).value]));
  const id = $("id").value;
  try { await api(id ? "PUT" : "POST", "/guests" + (id ? "/" + id : ""), g); resetForm(); loadGuests(); }
  catch (e) { show(e); }
}
async function deleteGuest(id) {
  if (!confirm("Удалить запись?")) return;
  try { await api("DELETE", "/guests/" + id); loadGuests(); } catch (e) { show(e); }
}

render();
