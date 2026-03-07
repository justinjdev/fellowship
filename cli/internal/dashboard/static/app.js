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
      await fetchAndRenderTidings();
      await fetchAndRenderProblems();
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
      await fetchAndRenderTidings();
      await fetchAndRenderProblems();
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

    card.innerHTML =
      "<h3>" + escapeHTML(quest.name || quest.worktree) + "</h3>" +
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

  // ── Event Stream ─────────────────────────────────

  var EVENT_TYPE_CLASSES = {
    gate_approved: "event-approved",
    phase_transition: "event-approved",
    gate_submitted: "event-submitted",
    lembas_completed: "event-submitted",
    metadata_updated: "event-submitted",
    gate_rejected: "event-rejected",
    file_modified: "event-neutral",
  };

  async function fetchAndRenderTidings() {
    try {
      var res = await fetch("/api/herald");
      if (!res.ok) return;
      var evts = await res.json();
      renderEventStream(evts);
    } catch (err) {
      // silently ignore event fetch errors
    }
  }

  function renderEventStream(evts) {
    var container = document.getElementById("event-stream");
    container.innerHTML = "";
    if (!evts || evts.length === 0) {
      var li = document.createElement("li");
      li.className = "event-item event-neutral";
      li.textContent = "No tidings recorded yet.";
      container.appendChild(li);
      return;
    }
    evts.forEach(function (evt) {
      var li = document.createElement("li");
      var cls = EVENT_TYPE_CLASSES[evt.type] || "event-neutral";
      li.className = "event-item " + cls;

      var timeStr = evt.timestamp;
      try {
        timeStr = new Date(evt.timestamp).toLocaleTimeString();
      } catch (e) {}

      var typeLabel = (evt.type || "").replace(/_/g, " ");
      li.innerHTML =
        '<span class="event-time">' + escapeHTML(timeStr) + "</span> " +
        '<span class="event-quest">' + escapeHTML(evt.quest || "") + "</span> " +
        '<span class="event-type-label">' + escapeHTML(typeLabel) + "</span>" +
        (evt.detail ? ' <span class="event-detail">' + escapeHTML(evt.detail) + "</span>" : "");
      container.appendChild(li);
    });
  }

  // ── Problems Banner ────────────────────────────

  async function fetchAndRenderProblems() {
    try {
      var res = await fetch("/api/herald/problems");
      if (!res.ok) return;
      var problems = await res.json();
      renderProblems(problems);
    } catch (err) {
      // silently ignore
    }
  }

  function renderProblems(problems) {
    var banner = document.getElementById("problems-banner");
    banner.innerHTML = "";
    if (!problems || problems.length === 0) {
      banner.style.display = "none";
      return;
    }
    banner.style.display = "block";
    problems.forEach(function (p) {
      var badge = document.createElement("div");
      badge.className = "problem-badge problem-" + p.severity;
      badge.innerHTML =
        '<span class="problem-severity">' + escapeHTML(p.severity) + "</span> " +
        '<span class="problem-quest">' + escapeHTML(p.quest) + "</span>: " +
        '<span class="problem-message">' + escapeHTML(p.message) + "</span>";
      banner.appendChild(badge);
    });
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
