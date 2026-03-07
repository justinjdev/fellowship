(function () {
  "use strict";

  const PHASES = ["Onboard", "Research", "Plan", "Implement", "Review", "Complete"];

  let prevStatus = null;
  let pollTimer = null;
  let patrolData = null;

  // ── Bootstrap ──────────────────────────────────

  async function init() {
    try {
      const data = await fetchStatus();
      patrolData = await fetchPatrol();
      render(data);
      const interval = (data.poll_interval || 5) * 1000;
      pollTimer = setInterval(poll, interval);
    } catch (err) {
      addActivity("Failed to connect: " + err.message);
    }
  }

  async function fetchStatus() {
    const res = await fetch("/api/status");
    if (!res.ok) throw new Error("status " + res.status);
    return res.json();
  }

  async function fetchPatrol() {
    try {
      const res = await fetch("/api/patrol");
      if (!res.ok) return null;
      return res.json();
    } catch (err) {
      return null;
    }
  }

  // ── Polling ────────────────────────────────────

  async function poll() {
    const dot = document.getElementById("poll-indicator");
    dot.classList.add("active");
    try {
      const data = await fetchStatus();
      patrolData = await fetchPatrol();
      detectChanges(data);
      render(data);
    } catch (err) {
      addActivity("Poll error: " + err.message);
    }
    setTimeout(() => dot.classList.remove("active"), 1000);
  }

  // ── Render ─────────────────────────────────────

  function render(status) {
    // Header
    document.getElementById("fellowship-name").textContent = status.name || "Fellowship";

    const quests = status.quests || [];
    const scouts = status.scouts || [];
    document.getElementById("quest-count").textContent = quests.length + " quest" + (quests.length !== 1 ? "s" : "");
    document.getElementById("scout-count").textContent = scouts.length + " scout" + (scouts.length !== 1 ? "s" : "");

    // Quest cards
    const container = document.getElementById("quest-cards");
    container.innerHTML = "";
    quests.forEach(function (q) {
      container.appendChild(renderCard(q));
    });

    // Scouts as simple cards
    scouts.forEach(function (s) {
      container.appendChild(renderScoutCard(s));
    });

    prevStatus = status;
  }

  function renderCard(quest) {
    const card = document.createElement("div");
    card.className = "quest-card" + (quest.gate_pending ? " pending" : "");

    const phaseIndex = PHASES.indexOf(quest.phase);

    // Progress bar segments
    let progressHTML = '<div class="progress-bar">';
    for (let i = 0; i < PHASES.length; i++) {
      progressHTML += '<div class="progress-segment' + (i <= phaseIndex ? " filled" : "") + '"></div>';
    }
    progressHTML += "</div>";

    var patrolHealth = getQuestHealth(quest.worktree);
    var badgeHTML = patrolHealth ? " " + renderHealthBadge(patrolHealth.health) : "";

    card.innerHTML =
      "<h3>" + escapeHTML(quest.name || quest.worktree) + badgeHTML + "</h3>" +
      '<div class="quest-phase">' + escapeHTML(quest.phase || "Unknown") + "</div>" +
      progressHTML;

    if (quest.gate_pending) {
      var actions = document.createElement("div");
      actions.className = "gate-actions";

      var approveBtn = document.createElement("button");
      approveBtn.className = "btn btn-approve";
      approveBtn.textContent = "Approve";
      approveBtn.addEventListener("click", function () { window.__approve(quest.worktree); });

      var rejectBtn = document.createElement("button");
      rejectBtn.className = "btn btn-reject";
      rejectBtn.textContent = "Reject";

      actions.appendChild(approveBtn);
      actions.appendChild(rejectBtn);
      card.appendChild(actions);

      var confirmDiv = document.createElement("div");
      confirmDiv.className = "reject-confirm";

      var span = document.createElement("span");
      span.textContent = "Are you sure?";
      confirmDiv.appendChild(span);

      var confirmBtn = document.createElement("button");
      confirmBtn.className = "btn btn-confirm-reject";
      confirmBtn.textContent = "Confirm Reject";
      confirmBtn.addEventListener("click", function () { window.__reject(quest.worktree); });
      confirmDiv.appendChild(confirmBtn);

      var cancelBtn = document.createElement("button");
      cancelBtn.className = "btn btn-cancel";
      cancelBtn.textContent = "Cancel";
      cancelBtn.addEventListener("click", function () { confirmDiv.classList.remove("visible"); });
      confirmDiv.appendChild(cancelBtn);

      rejectBtn.addEventListener("click", function () { confirmDiv.classList.add("visible"); });

      card.appendChild(confirmDiv);
    }

    return card;
  }

  function renderScoutCard(scout) {
    const card = document.createElement("div");
    card.className = "quest-card";
    card.innerHTML =
      "<h3>" + escapeHTML(scout.name || scout.worktree) + '</h3>' +
      '<div class="quest-phase">Scout</div>';
    return card;
  }

  // ── Gate Actions ───────────────────────────────

  window.__approve = async function (dir) {
    try {
      const res = await fetch("/api/gate/approve", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dir: dir }),
      });
      if (!res.ok) throw new Error("status " + res.status);
      addActivity("Approved gate for " + dir);
      poll();
    } catch (err) {
      addActivity("Approve failed: " + err.message);
    }
  };

  window.__reject = async function (dir) {
    try {
      const res = await fetch("/api/gate/reject", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dir: dir }),
      });
      if (!res.ok) throw new Error("status " + res.status);
      addActivity("Rejected gate for " + dir);
      poll();
    } catch (err) {
      addActivity("Reject failed: " + err.message);
    }
  };

  // ── Activity Feed ──────────────────────────────

  function addActivity(msg) {
    const feed = document.getElementById("activity-feed");
    const li = document.createElement("li");
    const time = new Date().toLocaleTimeString();
    li.textContent = time + " — " + msg;
    feed.prepend(li);
    // Keep feed manageable
    while (feed.children.length > 100) {
      feed.removeChild(feed.lastChild);
    }
  }

  function detectChanges(newStatus) {
    if (!prevStatus) return;

    const oldQuests = {};
    (prevStatus.quests || []).forEach(function (q) { oldQuests[q.worktree] = q; });

    (newStatus.quests || []).forEach(function (q) {
      const old = oldQuests[q.worktree];
      if (!old) {
        addActivity("New quest: " + (q.name || q.worktree));
        return;
      }
      if (old.phase !== q.phase) {
        addActivity((q.name || q.worktree) + ": " + old.phase + " → " + q.phase);
      }
      if (!old.gate_pending && q.gate_pending) {
        addActivity((q.name || q.worktree) + ": gate approval requested");
      }
      if (old.gate_pending && !q.gate_pending) {
        addActivity((q.name || q.worktree) + ": gate resolved");
      }
    });
  }

  // ── Patrol Helpers ─────────────────────────────

  var HEALTH_COLORS = {
    working: "#5a8a5a",
    stalled: "#c8a84e",
    zombie: "#8a4a4a",
    idle: "#6a6a6a",
    complete: "#5a8a5a"
  };

  var HEALTH_BG = {
    working: "rgba(90, 138, 90, 0.2)",
    stalled: "rgba(200, 168, 78, 0.2)",
    zombie: "rgba(138, 74, 74, 0.2)",
    idle: "rgba(106, 106, 106, 0.2)",
    complete: "rgba(90, 138, 90, 0.15)"
  };

  function getQuestHealth(worktree) {
    if (!patrolData || !patrolData.quests) return null;
    for (var i = 0; i < patrolData.quests.length; i++) {
      if (patrolData.quests[i].worktree === worktree) {
        return patrolData.quests[i];
      }
    }
    return null;
  }

  function renderHealthBadge(health) {
    if (!health) return "";
    var color = HEALTH_COLORS[health] || "#6a6a6a";
    var bg = HEALTH_BG[health] || "rgba(106, 106, 106, 0.2)";
    return '<span class="health-badge" style="' +
      "display:inline-block;padding:2px 8px;border-radius:4px;font-size:0.8rem;" +
      "border:1px solid " + color + ";background:" + bg + ";color:" + color + ";" +
      '">' + escapeHTML(health) + "</span>";
  }

  // ── Helpers ────────────────────────────────────

  function escapeHTML(str) {
    var div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
  }

  // ── Start ──────────────────────────────────────

  document.addEventListener("DOMContentLoaded", init);
})();
