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

    var errandProgressHTML = "";
    if (quest.errands_total > 0) {
      errandProgressHTML = '<div class="errand-progress">' +
        quest.errands_done + "/" + quest.errands_total + " errands done" +
        "</div>";
    }

    card.innerHTML =
      "<h3>" + escapeHTML(quest.name || quest.worktree) + "</h3>" +
      '<div class="quest-phase">' + escapeHTML(quest.phase || "Unknown") + "</div>" +
      progressHTML +
      errandProgressHTML;

    if (quest.errands_total > 0) {
      var errandDetails = document.createElement("div");
      errandDetails.className = "errand-details";
      errandDetails.style.display = "none";
      card.appendChild(errandDetails);

      card.style.cursor = "pointer";
      card.addEventListener("click", function (e) {
        if (e.target.tagName === "BUTTON") return;
        var details = card.querySelector(".errand-details");
        if (details.style.display === "none") {
          details.style.display = "block";
          loadErrandItems(quest.worktree, details);
        } else {
          details.style.display = "none";
        }
      });
    }

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
      if (q.errands_total > 0 && old.errands_done !== q.errands_done) {
        addActivity((q.name || q.worktree) + ": errand progress " + q.errands_done + "/" + q.errands_total);
      }
    });
  }

  // ── Errands ──────────────────────────────────

  async function loadErrandItems(worktree, container) {
    try {
      var encoded = btoa(worktree).replace(/\+/g, '-').replace(/\//g, '_');
      var res = await fetch("/api/errand/" + encoded);
      if (!res.ok) {
        container.innerHTML = "<p>No errands available.</p>";
        return;
      }
      var data = await res.json();
      var items = data.items || [];
      if (items.length === 0) {
        container.innerHTML = "<p>No errands.</p>";
        return;
      }
      var html = '<ul class="errand-item-list">';
      for (var i = 0; i < items.length; i++) {
        var item = items[i];
        var badge = '<span class="status-badge status-' + escapeHTML(item.status) + '">' + escapeHTML(item.status) + "</span>";
        html += "<li>" + badge + " <strong>" + escapeHTML(item.id) + "</strong> " + escapeHTML(item.description) + "</li>";
      }
      html += "</ul>";
      container.innerHTML = html;
    } catch (err) {
      container.innerHTML = "<p>Failed to load errands.</p>";
    }
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
