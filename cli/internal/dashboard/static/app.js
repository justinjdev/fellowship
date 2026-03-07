(function () {
  "use strict";

  const PHASES = ["Onboard", "Research", "Plan", "Implement", "Review", "Complete"];

  let prevStatus = null;
  let pollTimer = null;

  // ── Bootstrap ──────────────────────────────────

  async function init() {
    try {
      const data = await fetchStatus();
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

  // ── Polling ────────────────────────────────────

  async function poll() {
    const dot = document.getElementById("poll-indicator");
    dot.classList.add("active");
    try {
      const data = await fetchStatus();
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

    let gateHTML = "";
    if (quest.gate_pending) {
      gateHTML =
        '<div class="gate-actions">' +
          '<button class="btn btn-approve" onclick="window.__approve(\'' + escapeAttr(quest.worktree) + '\')">Approve</button>' +
          '<button class="btn btn-reject" onclick="window.__showReject(this)">Reject</button>' +
        "</div>" +
        '<div class="reject-confirm" id="reject-' + escapeAttr(quest.worktree) + '">' +
          '<span>Are you sure?</span>' +
          '<button class="btn btn-confirm-reject" onclick="window.__reject(\'' + escapeAttr(quest.worktree) + '\')">Confirm Reject</button>' +
          '<button class="btn btn-cancel" onclick="window.__hideReject(this)">Cancel</button>' +
        "</div>";
    }

    card.innerHTML =
      "<h3>" + escapeHTML(quest.name || quest.worktree) + "</h3>" +
      '<div class="quest-phase">' + escapeHTML(quest.phase || "Unknown") + "</div>" +
      progressHTML +
      gateHTML;

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

  window.__showReject = function (btn) {
    const confirm = btn.parentElement.nextElementSibling;
    if (confirm) confirm.classList.add("visible");
  };

  window.__hideReject = function (btn) {
    btn.closest(".reject-confirm").classList.remove("visible");
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

  // ── Helpers ────────────────────────────────────

  function escapeHTML(str) {
    var div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
  }

  function escapeAttr(str) {
    return str.replace(/&/g, "&amp;").replace(/'/g, "&#39;").replace(/"/g, "&quot;");
  }

  // ── Start ──────────────────────────────────────

  document.addEventListener("DOMContentLoaded", init);
})();
