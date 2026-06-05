/**
 * Discord alert bot.
 *
 * Listens on PORT for Alertmanager webhook POSTs at /alert, formats each alert
 * into a Discord message and DMs it to DEVELOPER_DISCORD_ID.
 *
 * Required environment variables:
 *   DISCORD_BOT_TOKEN     - bot token from discord.com/developers
 *   DEVELOPER_DISCORD_ID  - the Discord user ID to DM
 *
 * Optional:
 *   PORT                  - HTTP port (default 9094)
 */

const { Client, GatewayIntentBits, Partials } = require("discord.js");
const express = require("express");

const PORT = parseInt(process.env.PORT || "9094", 10);
const TOKEN = process.env.DISCORD_BOT_TOKEN;
const DEVELOPER_DISCORD_ID = process.env.DEVELOPER_DISCORD_ID;
const botEnabled = Boolean(TOKEN && DEVELOPER_DISCORD_ID);
const disabledReason = !TOKEN
  ? "DISCORD_BOT_TOKEN is not configured"
  : !DEVELOPER_DISCORD_ID
    ? "DEVELOPER_DISCORD_ID is not configured"
    : null;

const client = new Client({
  intents: [GatewayIntentBits.Guilds, GatewayIntentBits.DirectMessages],
  partials: [Partials.Channel],
});

let botReady = false;
if (botEnabled) {
  client.once("ready", () => {
    console.log(`[bot] logged in as ${client.user.tag}`);
    botReady = true;
  });
  client.on("error", (err) => console.error("[bot] client error:", err));
  client.login(TOKEN).catch((err) => {
    console.error("[bot] login failed:", err);
    process.exit(1);
  });
} else {
  console.warn(`[bot] disabled: ${disabledReason}`);
}

function formatAlert(alert, status) {
  const severity = alert.labels?.severity || "info";
  const emoji = status === "resolved" ? "✅" : severity === "critical" ? "🔴" : "⚠️";
  const statusLabel = status === "resolved" ? "RESOLVED" : "FIRING";
  const name = alert.labels?.alertname || "Alert";
  const summary = alert.annotations?.summary || "";
  const description = alert.annotations?.description || "";

  const lines = [`${emoji} **[${statusLabel}] ${name}**`];
  if (summary) lines.push(summary);
  if (description) lines.push(description);

  const svc = alert.labels?.service || alert.labels?.service_name;
  if (svc) lines.push(`Service: \`${svc}\``);

  const component = alert.labels?.component;
  if (component) lines.push(`Component: \`${component}\``);

  if (alert.startsAt) lines.push(`Started: ${alert.startsAt}`);
  if (status === "resolved" && alert.endsAt) lines.push(`Ended: ${alert.endsAt}`);

  return lines.join("\n");
}

const app = express();
app.use(express.json({ limit: "1mb" }));

app.get("/health", (_req, res) => {
  res.status(200).json({
    enabled: botEnabled,
    ready: botEnabled ? botReady : false,
    reason: disabledReason,
  });
});

app.post("/alert", async (req, res) => {
  if (!botEnabled) {
    console.warn("[bot] dropping alert because the bot is disabled");
    return res.status(202).json({ status: "disabled", reason: disabledReason });
  }

  if (!botReady) {
    return res.status(503).json({ error: "bot not ready" });
  }

  const payload = req.body || {};
  const alerts = Array.isArray(payload.alerts) ? payload.alerts : [];
  const status = payload.status || "firing";

  if (alerts.length === 0) {
    return res.status(400).json({ error: "no alerts in payload" });
  }

  try {
    const user = await client.users.fetch(DEVELOPER_DISCORD_ID);
    for (const alert of alerts) {
      const message = formatAlert(alert, alert.status || status);
      await user.send(message);
    }
    console.log(`[bot] sent ${alerts.length} alert(s) to ${DEVELOPER_DISCORD_ID}`);
    res.json({ status: "ok", delivered: alerts.length });
  } catch (err) {
    console.error("[bot] failed to send DM:", err);
    res.status(500).json({ error: err.message });
  }
});

app.listen(PORT, () => {
  console.log(`[bot] HTTP server listening on ${PORT}`);
});
