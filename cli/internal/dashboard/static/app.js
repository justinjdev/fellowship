(function () {
  "use strict";

  const PHASES = ["Onboard", "Research", "Plan", "Implement", "Review", "Complete"];

  let prevStatus = null;
  let pollTimer = null;
  let eaglesData = null;

  // ── Bootstrap ──────────────────────────────────

  async function init() {
    try {
      const data = await fetchStatus();
      eaglesData = await fetchEagles();
      render(data);
      await fetchAndRenderTidings();
      await fetchAndRenderBulletin();
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

  async function fetchEagles() {
    try {
      const res = await fetch("/api/eagles");
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
      eaglesData = await fetchEagles();
      detectChanges(data);
      render(data);
      await fetchAndRenderTidings();
      await fetchAndRenderBulletin();
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
    var activeQuests = quests.filter(function (q) { return q.status !== "completed" && q.status !== "cancelled"; });
    var doneQuests = quests.filter(function (q) { return q.status === "completed" || q.status === "cancelled"; });
    var questCountText = quests.length + " quest" + (quests.length !== 1 ? "s" : "");
    if (doneQuests.length > 0) {
      questCountText += " (" + doneQuests.length + " done)";
    }
    document.getElementById("quest-count").textContent = questCountText;
    document.getElementById("scout-count").textContent = scouts.length + " scout" + (scouts.length !== 1 ? "s" : "");

    // Quest cards — group by company
    const container = document.getElementById("quest-cards");
    container.innerHTML = "";
    var companies = status.companies || [];
    var rendered = {};

    companies.forEach(function (c) {
      var companyQuests = activeQuests.filter(function (q) {
        return c.quests && c.quests.indexOf(q.name) !== -1;
      });
      var companyScouts = scouts.filter(function (s) {
        return c.scouts && c.scouts.indexOf(s.name) !== -1;
      });
      if (companyQuests.length === 0 && companyScouts.length === 0) return;

      container.appendChild(renderCompanyHeader(c, companyQuests));

      companyQuests.forEach(function (q) {
        container.appendChild(renderCard(q));
        rendered[q.name] = true;
      });
      companyScouts.forEach(function (s) {
        container.appendChild(renderScoutCard(s));
        rendered[s.name] = true;
      });
    });

    // Ungrouped active quests and scouts
    var ungroupedQuests = activeQuests.filter(function (q) { return !rendered[q.name]; });
    var ungroupedScouts = scouts.filter(function (s) { return !rendered[s.name]; });
    if (ungroupedQuests.length > 0 || ungroupedScouts.length > 0) {
      if (companies.length > 0) {
        var ungroupedHeader = document.createElement("div");
        ungroupedHeader.className = "company-header";
        ungroupedHeader.innerHTML = "<h2>Ungrouped</h2>";
        container.appendChild(ungroupedHeader);
      }
      ungroupedQuests.forEach(function (q) {
        container.appendChild(renderCard(q));
      });
      ungroupedScouts.forEach(function (s) {
        container.appendChild(renderScoutCard(s));
      });
    }

    // Done quests section
    if (doneQuests.length > 0) {
      var doneSection = document.createElement("div");
      doneSection.className = "done-section";

      var doneHeader = document.createElement("div");
      doneHeader.className = "done-section-header";
      doneHeader.innerHTML = "<h2>Done</h2>" +
        '<span class="done-count">' + doneQuests.length + " quest" + (doneQuests.length !== 1 ? "s" : "") + "</span>";
      doneHeader.style.cursor = "pointer";

      var doneCards = document.createElement("div");
      doneCards.className = "done-cards";

      doneQuests.forEach(function (q) {
        doneCards.appendChild(renderCard(q));
      });

      doneHeader.addEventListener("click", function () {
        doneCards.classList.toggle("collapsed");
        doneHeader.classList.toggle("collapsed");
      });

      doneSection.appendChild(doneHeader);
      doneSection.appendChild(doneCards);
      container.appendChild(doneSection);
    }

    prevStatus = status;
  }

  function renderCompanyHeader(company, companyQuests) {
    var header = document.createElement("div");
    header.className = "company-header";

    var implementPlus = 0;
    var total = (company.quests || []).length + (company.scouts || []).length;
    var hasPending = false;

    companyQuests.forEach(function (q) {
      var idx = PHASES.indexOf(q.phase);
      if (idx >= 3) implementPlus++; // Implement+
      if (q.gate_pending) hasPending = true;
    });

    var summary = implementPlus + "/" + total + " quests in Implement+";
    header.innerHTML = "<h2>" + escapeHTML(company.name) + "</h2>" +
      '<span class="company-summary">' + escapeHTML(summary) + "</span>";

    if (hasPending) {
      var approveAllBtn = document.createElement("button");
      approveAllBtn.className = "btn btn-approve";
      approveAllBtn.textContent = "Approve All";
      approveAllBtn.addEventListener("click", function () {
        window.__approveCompany(company.name);
      });
      header.appendChild(approveAllBtn);
    }

    return header;
  }

  function renderCard(quest) {
    const card = document.createElement("div");
    var isDone = quest.status === "completed" || quest.status === "cancelled";
    card.className = "quest-card" + (quest.gate_pending ? " pending" : "") + (isDone ? " " + quest.status : "");

    const phaseIndex = PHASES.indexOf(quest.phase);

    // Progress bar segments
    let progressHTML = '<div class="progress-bar">';
    for (let i = 0; i < PHASES.length; i++) {
      progressHTML += '<div class="progress-segment' + (i <= phaseIndex ? " filled" : "") + '"></div>';
    }
    progressHTML += "</div>";

    var eaglesHealth = getQuestHealth(quest.worktree);
    var badgeHTML = eaglesHealth ? " " + renderHealthBadge(eaglesHealth.health) : "";

    var statusBadgeHTML = "";
    if (isDone) {
      var statusLabel = quest.status === "completed" ? "completed" : "cancelled";
      var statusClass = quest.status === "completed" ? "status-done" : "status-blocked";
      statusBadgeHTML = ' <span class="status-badge ' + statusClass + '">' + escapeHTML(statusLabel) + "</span>";
    }

    var errandProgressHTML = "";
    if (quest.errands_total > 0) {
      errandProgressHTML = '<div class="errand-progress">' +
        quest.errands_done + "/" + quest.errands_total + " errands done" +
        "</div>";
    }

    card.innerHTML =
      "<h3>" + escapeHTML(quest.name || quest.worktree) + badgeHTML + statusBadgeHTML + "</h3>" +
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

  window.__approveCompany = async function (name) {
    try {
      var res = await fetch("/api/company/" + encodeURIComponent(name) + "/approve", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });
      if (!res.ok) throw new Error("status " + res.status);
      var data = await res.json();
      addActivity("Company '" + name + "': approved " + data.approved.length + " gate(s)");
      poll();
    } catch (err) {
      addActivity("Company approve failed: " + err.message);
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

  // ── Eagles Helpers ─────────────────────────────

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
    if (!eaglesData || !eaglesData.quests) return null;
    for (var i = 0; i < eaglesData.quests.length; i++) {
      if (eaglesData.quests[i].worktree === worktree) {
        return eaglesData.quests[i];
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

  // ── Bulletin Board ───────────────────────────────

  async function fetchAndRenderBulletin() {
    try {
      var res = await fetch("/api/bulletin");
      if (!res.ok) return;
      var entries = await res.json();
      renderBulletin(entries);
    } catch (err) {
      // silently ignore
    }
  }

  function renderBulletin(entries) {
    var container = document.getElementById("bulletin-entries");
    var section = document.getElementById("bulletin-section");
    if (!container || !section) return;

    if (!entries || entries.length === 0) {
      section.style.display = "none";
      return;
    }
    section.style.display = "block";
    container.innerHTML = "";

    // Group by topic
    var byTopic = {};
    entries.forEach(function (e) {
      var topic = e.topic || "general";
      if (!byTopic[topic]) byTopic[topic] = [];
      byTopic[topic].push(e);
    });

    Object.keys(byTopic).forEach(function (topic) {
      var group = document.createElement("div");
      group.className = "bulletin-topic-group";

      var header = document.createElement("div");
      header.className = "bulletin-topic-header";
      header.textContent = topic;
      group.appendChild(header);

      byTopic[topic].forEach(function (e) {
        var item = document.createElement("div");
        item.className = "bulletin-item";

        var timeStr = e.ts;
        try {
          timeStr = new Date(e.ts).toLocaleTimeString();
        } catch (err) {}

        var filesStr = "";
        if (e.files && e.files.length > 0) {
          filesStr = ' <span class="bulletin-files">' + escapeHTML(e.files.join(", ")) + "</span>";
        }

        item.innerHTML =
          '<span class="bulletin-time">' + escapeHTML(timeStr) + "</span> " +
          '<span class="bulletin-quest">' + escapeHTML(e.quest || "") + "</span> " +
          '<span class="bulletin-discovery">' + escapeHTML(e.discovery || "") + "</span>" +
          filesStr;
        group.appendChild(item);
      });

      container.appendChild(group);
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
    if (!container) return;
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
    if (!banner) return;
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
