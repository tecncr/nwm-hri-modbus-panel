import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import {
  Activity,
  AlertTriangle,
  ArrowDownLeft,
  ArrowUpRight,
  Cable,
  Check,
  ChevronRight,
  CircleHelp,
  Clock3,
  Droplets,
  Gauge,
  LayoutDashboard,
  RefreshCw,
  Save,
  Search,
  Settings2,
  ShieldCheck,
  Waves,
  Wrench,
} from "lucide-react";
import { Badge, Button, Card, Field, Input } from "./components/ui";
import {
  combine32,
  decodeClock,
  encodeClock,
  formatVolume,
  hex,
  split32,
} from "./lib";

type Status = {
  httpAddr: string;
  serialPort: string;
  baudRate: number;
  dataBits: number;
  parity: string;
  stopBits: number;
  timeoutMs: number;
  protocol: string;
  defaultSlave: number;
  version: string;
  functions: string[];
};
type Register = {
  address: number;
  name: string;
  access: string;
  group: string;
  note?: string;
};
type Transaction = {
  at: string;
  request: string;
  response?: string;
  durationMs: number;
  error?: string;
};
type Tab = "overview" | "configuration" | "registers" | "diagnostics";
type RegisterResult = { slave: number; address: number; values: number[] };

const api = {
  get: async <T,>(url: string): Promise<T> => {
    const response = await fetch(url);
    const data = await response.json();
    if (!response.ok) throw new Error(data.error ?? response.statusText);
    return data as T;
  },
  post: async <T,>(url: string, body: unknown): Promise<T> => {
    const response = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error ?? response.statusText);
    return data as T;
  },
};

const nav: { id: Tab; label: string; icon: typeof LayoutDashboard }[] = [
  { id: "overview", label: "Overview", icon: LayoutDashboard },
  { id: "configuration", label: "Configuration", icon: Settings2 },
  { id: "registers", label: "Register browser", icon: Search },
  { id: "diagnostics", label: "Diagnostics", icon: Activity },
];

export default function App() {
  const [page, setPage] = useState<Tab>("overview");
  const [status, setStatus] = useState<Status | null>(null);
  const [registers, setRegisters] = useState<Register[]>([]);
  const [values, setValues] = useState<Record<number, number>>({});
  const [slave, setSlave] = useState(15);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{
    text: string;
    error?: boolean;
  } | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  const refreshTransactions = useCallback(async () => {
    try {
      setTransactions(await api.get<Transaction[]>("/api/transactions"));
    } catch {
      /* diagnostics are optional */
    }
  }, []);

  useEffect(() => {
    let live = true;
    Promise.all([
      api.get<Status>("/api/status"),
      api.get<Register[]>("/api/registers"),
      api.get<Transaction[]>("/api/transactions"),
    ])
      .then(([s, r, t]) => {
        if (!live) return;
        setStatus(s);
        setSlave(s.defaultSlave);
        setRegisters(r);
        setTransactions(t);
      })
      .catch((error) => {
        if (live) setNotice({ text: error.message, error: true });
      });
    return () => {
      live = false;
    };
  }, []);

  const act = async (work: () => Promise<void>, success?: string) => {
    setBusy(true);
    setNotice(null);
    try {
      await work();
      if (success) setNotice({ text: success });
    } catch (error) {
      setNotice({
        text: error instanceof Error ? error.message : String(error),
        error: true,
      });
    } finally {
      setBusy(false);
      await refreshTransactions();
    }
  };

  const read = async (address: number, count: number, targetSlave = slave) => {
    const result = await api.post<RegisterResult>("/api/read", {
      slave: targetSlave,
      address,
      count,
    });
    setValues((current) => {
      const updated = { ...current };
      result.values.forEach((value, index) => {
        updated[result.address + index] = value;
      });
      return updated;
    });
    setLastUpdated(new Date());
    return result.values;
  };

  const refreshMeter = () =>
    act(async () => {
      await read(0x0000, 0x0016);
      await read(0x0019, 3);
    }, "Meter data refreshed");

  const title = nav.find((item) => item.id === page)?.label ?? "Overview";
  const total = combine32(values[1], values[2]);
  const forward = combine32(values[3], values[4]);
  const reverse = combine32(values[5], values[6]);
  const clock = decodeClock([values[7], values[8], values[9]]);

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <Droplets size={21} />
          </div>
          <div>
            <strong>
              NWM<span>·</span>HRI
            </strong>
            <small>METER CONSOLE</small>
          </div>
        </div>
        <div className="device-chip">
          <span className="device-dot" />
          <div>
            <strong>Water meter</strong>
            <small>RTU field device</small>
          </div>
          <ChevronRight size={15} />
        </div>
        <div className="nav-label">WORKSPACE</div>
        <nav>
          {nav.map((item) => {
            const Icon = item.icon;
            return (
              <button
                key={item.id}
                className={`nav-item ${page === item.id ? "active" : ""}`}
                onClick={() => setPage(item.id)}
              >
                <Icon size={17} />
                <span>{item.label}</span>
                {item.id === "diagnostics" && (
                  <span className="nav-count">{transactions.length}</span>
                )}
              </button>
            );
          })}
        </nav>
        <div className="sidebar-bottom">
          <div className="connection-card">
            <div className="connection-head">
              <span className="pulse-dot" /> MODBUS RTU
            </div>
            <div className="connection-port">
              {status?.serialPort ?? "Loading serial port…"}
            </div>
            <div className="connection-settings">
              {status?.baudRate ?? "—"} baud <span>·</span> 8N1
            </div>
          </div>
          <div className="sidebar-foot">
            <ShieldCheck size={14} /> Local device interface
          </div>
        </div>
      </aside>

      <main className="main-area">
        <header className="topbar">
          <div className="breadcrumbs">
            <span>Workspace</span>
            <ChevronRight size={14} />
            <strong>{title}</strong>
          </div>
          <div className="top-actions">
            <div className="slave-control">
              <span>SLAVE ID</span>
              <Input
                aria-label="Modbus slave ID"
                type="number"
                min="1"
                max="247"
                value={slave}
                onChange={(event) => setSlave(Number(event.target.value))}
              />
            </div>
            <Button
              variant="secondary"
              className="refresh-top"
              onClick={refreshMeter}
              disabled={busy}
            >
              <RefreshCw size={15} className={busy ? "spin" : ""} /> Refresh
              data
            </Button>
            <div className="avatar">NW</div>
          </div>
        </header>
        <div className="page-content">
          <div className="page-heading">
            <div>
              <div className="eyebrow">FIELD DEVICE / NWM-HRI ENCODER</div>
              <h1>{page === "overview" ? "Meter overview" : title}</h1>
              <p>
                {page === "overview"
                  ? "Live consumption and device health at a glance."
                  : page === "configuration"
                    ? "Update identity, meter parameters, baselines, and the device clock."
                    : page === "registers"
                      ? "Inspect the complete register map or send raw Modbus requests."
                      : "Review RTU traffic, communication settings, and device diagnostics."}
              </p>
            </div>
            <Badge tone={lastUpdated ? "green" : "amber"}>
              {lastUpdated ? "Data loaded" : "Awaiting read"}
            </Badge>
          </div>
          {notice && (
            <div
              className={`notice ${notice.error ? "notice-error" : "notice-success"}`}
            >
              <span>
                {notice.error ? (
                  <AlertTriangle size={16} />
                ) : (
                  <Check size={16} />
                )}
              </span>
              {notice.text}
              {notice.error &&
                notice.text.includes("serial response timeout") && (
                  <small className="notice-hint">
                    No reply at this slave ID. Check the selected address or use
                    Diagnostics → Scan slave IDs.
                  </small>
                )}
              <button aria-label="Dismiss" onClick={() => setNotice(null)}>
                ×
              </button>
            </div>
          )}
          {page === "overview" && (
            <Overview
              status={status}
              values={values}
              total={total}
              forward={forward}
              reverse={reverse}
              clock={clock}
              busy={busy}
              lastUpdated={lastUpdated}
              refresh={refreshMeter}
              registers={registers}
            />
          )}
          {page === "configuration" && (
            <Configuration
              slave={slave}
              setSlave={setSlave}
              setValues={setValues}
              busy={busy}
              act={act}
              onRead={read}
              values={values}
            />
          )}
          {page === "registers" && (
            <RegisterBrowser
              slave={slave}
              busy={busy}
              act={act}
              read={read}
              values={values}
              registers={registers}
            />
          )}
          {page === "diagnostics" && (
            <Diagnostics
              status={status}
              slave={slave}
              setSlave={setSlave}
              transactions={transactions}
              values={values}
              busy={busy}
              act={act}
              refreshTransactions={refreshTransactions}
              read={read}
            />
          )}
          <footer className="page-footer">
            <span>
              NWM-HRI Modbus panel <span className="footer-sep">/</span> RTU 8N1
            </span>
            <span>
              {lastUpdated
                ? `Updated ${lastUpdated.toLocaleTimeString()}`
                : "No meter read yet"}
            </span>
            <span>{status?.version || "development build"}</span>
          </footer>
        </div>
      </main>
    </div>
  );
}

function Overview({
  status,
  values,
  total,
  forward,
  reverse,
  clock,
  busy,
  lastUpdated,
  refresh,
  registers,
}: {
  status: Status | null;
  values: Record<number, number>;
  total: number | null;
  forward: number | null;
  reverse: number | null;
  clock: string | null;
  busy: boolean;
  lastUpdated: Date | null;
  refresh: () => void;
  registers: Register[];
}) {
  const battery = values[0x0e];
  const failure = values[0x0f];
  const outputPulse = values[0x11];
  const firmware =
    values[0x0b] !== undefined && values[0x0c] !== undefined
      ? `${values[0x0b]} · ${hex(values[0x0c])}`
      : "—";
  const directionalTotal = (forward ?? 0) + (reverse ?? 0);
  const forwardPercent =
    directionalTotal > 0 ? ((forward ?? 0) / directionalTotal) * 100 : 65;
  const flowBackground =
    forward === null || reverse === null
      ? "#edf1f5"
      : `conic-gradient(#5b89df 0 ${forwardPercent}%, #e4a940 ${forwardPercent}% 100%)`;
  return (
    <>
      <div className="overview-toolbar">
        <div className="reading-status">
          <span className="reading-indicator" />{" "}
          {lastUpdated
            ? "Last read succeeded"
            : "Read the meter to load live values"}{" "}
          <span className="muted-dot">·</span>{" "}
          {lastUpdated
            ? lastUpdated.toLocaleTimeString()
            : "Modbus data is not cached"}
        </div>
        <Button onClick={refresh} disabled={busy}>
          <RefreshCw size={15} className={busy ? "spin" : ""} /> Read meter
        </Button>
      </div>
      <div className="metric-grid">
        <Metric
          icon={<Waves size={17} />}
          label="Total flow"
          value={total === null ? "—" : total.toLocaleString()}
          unit="raw counts"
          accent="blue"
          foot="Registers 0x0001–0x0002"
          converted={formatVolume(total, outputPulse)}
        />
        <Metric
          icon={<ArrowUpRight size={17} />}
          label="Forward flow"
          value={forward === null ? "—" : forward.toLocaleString()}
          unit="raw counts"
          accent="green"
          foot="Registers 0x0003–0x0004"
          converted={formatVolume(forward, outputPulse)}
        />
        <Metric
          icon={<ArrowDownLeft size={17} />}
          label="Reverse flow"
          value={reverse === null ? "—" : reverse.toLocaleString()}
          unit="raw counts"
          accent="amber"
          foot="Registers 0x0005–0x0006"
          converted={formatVolume(reverse, outputPulse)}
        />
      </div>
      <div className="overview-columns">
        <Card className="flow-card">
          <div className="card-heading">
            <div>
              <span className="section-kicker">METER ACTIVITY</span>
              <h2>Flow snapshot</h2>
            </div>
            <span className="icon-box blue-box">
              <Gauge size={17} />
            </span>
          </div>
          <div className="flow-visual">
            <div className="flow-ring" style={{ background: flowBackground }}>
              <div className="flow-ring-inner">
                <Waves size={22} />
                <strong>{total === null ? "—" : shortNumber(total)}</strong>
                <span>RAW COUNTS</span>
              </div>
            </div>
            <div className="flow-legend">
              <div>
                <i className="legend-blue" />
                <span>Forward</span>
                <strong>{forward?.toLocaleString() ?? "—"}</strong>
              </div>
              <div>
                <i className="legend-amber" />
                <span>Reverse</span>
                <strong>{reverse?.toLocaleString() ?? "—"}</strong>
              </div>
              <div className="legend-divider" />
              <div>
                <span>Flow registers</span>
                <strong>4 words</strong>
              </div>
            </div>
          </div>
          <div className="flow-note">
            <CircleHelp size={14} /> Converted volumes use the output pulse
            equivalent (register 0x0011, L/pulse); the raw 32-bit counts remain
            shown above.
          </div>
        </Card>
        <Card className="health-card">
          <div className="card-heading">
            <div>
              <span className="section-kicker">DEVICE CHECK</span>
              <h2>Status & metadata</h2>
            </div>
            <span className="icon-box green-box">
              <ShieldCheck size={17} />
            </span>
          </div>
          <div className="health-list">
            <HealthRow
              label="Serial settings"
              value={`${status?.baudRate ?? "—"} baud · 8N1`}
              tone="neutral"
            />
            <HealthRow
              label="Device clock"
              value={clock ?? "Not read"}
              tone={clock ? "green" : "neutral"}
            />
            <HealthRow
              label="Output pulse equivalent"
              value={outputPulse ? `${outputPulse} L/pulse` : "Not read"}
              tone={outputPulse ? "green" : "neutral"}
            />
            <HealthRow
              label="Battery status"
              value={
                battery === undefined ? "Not read" : `${hex(battery)} · raw`
              }
              tone="neutral"
            />
            <HealthRow
              label="Failure register"
              value={
                failure === undefined ? "Not read" : `${hex(failure)} · raw`
              }
              tone={failure ? "amber" : "green"}
            />
            <HealthRow
              label="Firmware registers"
              value={firmware}
              tone="neutral"
            />
          </div>
          <div className="health-bottom">
            <span>
              <Clock3 size={14} /> Poll on demand
            </span>
            <span>
              <Cable size={14} /> {registers.length} mapped registers
            </span>
          </div>
        </Card>
      </div>
      <div className="section-header">
        <div>
          <span className="section-kicker">QUICK ACCESS</span>
          <h2>Common register groups</h2>
        </div>
      </div>
      <div className="quick-grid">
        {[
          ["Flow counters", "0x0001—0x0006", "Cumulative · forward · reverse"],
          ["Clock", "0x0007—0x0009", "Device real-time clock"],
          ["Configuration", "0x000D—0x0011", "Thresholds and pulse values"],
          ["Baselines", "0x0012—0x0015", "Forward and reverse presets"],
        ].map(([name, address, description], index) => (
          <div className="quick-card" key={name}>
            <span className={`quick-icon quick-${index}`}>
              <Wrench size={15} />
            </span>
            <div>
              <strong>{name}</strong>
              <span>{description}</span>
            </div>
            <code>{address}</code>
          </div>
        ))}
      </div>
    </>
  );
}

function Metric({
  icon,
  label,
  value,
  unit,
  accent,
  foot,
  converted,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  unit: string;
  accent: string;
  foot: string;
  converted: string | null;
}) {
  return (
    <Card className={`metric-card metric-${accent}`}>
      <div className="metric-top">
        <span className="metric-icon">{icon}</span>
        <span className="metric-label">{label}</span>
        <span className="metric-menu">···</span>
      </div>
      <div className="metric-value">
        {value}
        <span>{unit}</span>
      </div>
      <div className="metric-foot">
        <span>{foot}</span>
        <strong>
          {converted ? `≈ ${converted}` : "Read output pulse scale to convert"}
        </strong>
      </div>
    </Card>
  );
}

function HealthRow({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "green" | "amber" | "neutral";
}) {
  return (
    <div className="health-row">
      <span>{label}</span>
      <div>
        <span className={`health-dot ${tone}`} />
        <strong>{value}</strong>
      </div>
    </div>
  );
}

function Configuration({
  slave,
  setSlave,
  setValues,
  busy,
  act,
  onRead,
  values,
}: {
  slave: number;
  setSlave: (value: number) => void;
  setValues: Dispatch<SetStateAction<Record<number, number>>>;
  busy: boolean;
  act: (work: () => Promise<void>, success?: string) => Promise<void>;
  onRead: (address: number, count: number, slave?: number) => Promise<number[]>;
  values: Record<number, number>;
}) {
  const [addressValue, setAddressValue] = useState("24");
  const [threshold, setThreshold] = useState("");
  const [basePulse, setBasePulse] = useState("");
  const [outputPulse, setOutputPulse] = useState("");
  const [forwardBase, setForwardBase] = useState("");
  const [reverseBase, setReverseBase] = useState("");
  const [clockValue, setClockValue] = useState(() => {
    const now = new Date();
    return new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
      .toISOString()
      .slice(0, 16);
  });

  useEffect(() => {
    setAddressValue(String(values[0] ?? (slave === 247 ? 1 : slave + 1)));
    if (values[0x0d] !== undefined) setThreshold(String(values[0x0d]));
    if (values[0x10] !== undefined) setBasePulse(String(values[0x10]));
    if (values[0x11] !== undefined) setOutputPulse(String(values[0x11]));
    const forward = combine32(values[0x12], values[0x13]);
    if (forward !== null) setForwardBase(String(forward));
    const reverse = combine32(values[0x14], values[0x15]);
    if (reverse !== null) setReverseBase(String(reverse));
  }, [values, slave]);

  const readConfig = () =>
    act(async () => {
      await onRead(0, 0x16);
    }, "Configuration registers read");
  const writeOne = (
    address: number,
    input: string,
    label: string,
    minimum = 0,
    maximum = 0xffff,
  ) =>
    act(async () => {
      const value = Number(input);
      if (!Number.isInteger(value) || value < minimum || value > maximum)
        throw new Error(
          `${label} must be an integer from ${minimum} to ${maximum}`,
        );
      if (address === 0x10 && (!outputPulse || Number(outputPulse) <= value))
        throw new Error(
          "Read the current output pulse first and keep it greater than the base meter pulse",
        );
      if (address === 0x11 && (!basePulse || value <= Number(basePulse)))
        throw new Error(
          "Output pulse equivalent must be greater than the base meter pulse",
        );
      await api.post("/api/write-single", { slave, address, value });
      await onRead(address, 1);
    }, `${label} updated`);
  const writeBaselines = () =>
    act(async () => {
      const f = split32(Number(forwardBase));
      const r = split32(Number(reverseBase));
      await api.post("/api/write-multiple", {
        slave,
        address: 0x12,
        values: [...f, ...r],
      });
      await onRead(0x12, 4);
    }, "Flow baselines written");
  const writeClock = () =>
    act(async () => {
      await api.post("/api/write-multiple", {
        slave,
        address: 0x07,
        values: encodeClock(clockValue),
      });
      await onRead(0x07, 3);
    }, "Device clock updated");

  return (
    <div className="config-layout">
      <div className="config-main">
        <Card className="config-card">
          <CardTitle
            icon={<Settings2 size={17} />}
            kicker="IDENTITY"
            title="Meter address"
            description="Change the Modbus slave ID stored in register 0x0000."
          />
          <div className="config-fields config-address">
            <Field label="Current slave ID">
              <Input
                type="number"
                min="1"
                max="247"
                value={slave}
                onChange={(e) => setSlave(Number(e.target.value))}
              />
            </Field>
            <span className="field-arrow">
              <ArrowUpRight size={16} />
            </span>
            <Field label="New device address">
              <Input
                type="number"
                min="1"
                max="247"
                value={addressValue}
                onChange={(e) => setAddressValue(e.target.value)}
              />
            </Field>
            <Button
              disabled={busy}
              onClick={() =>
                act(async () => {
                  const next = Number(addressValue);
                  if (!Number.isInteger(next) || next < 1 || next > 247)
                    throw new Error("Address must be 1..247");
                  await api.post("/api/write-single", {
                    slave,
                    address: 0,
                    value: next,
                  });
                  setValues((current) => ({ ...current, 0: next }));
                  setSlave(next);
                }, "Address changed — update your connection if needed")
              }
            >
              <Save size={14} /> Save address
            </Button>
          </div>
          <div className="inline-warning">
            <AlertTriangle size={14} /> Changing the address immediately moves
            the meter to the new slave ID.
          </div>
        </Card>
        <Card className="config-card">
          <CardTitle
            icon={<Gauge size={17} />}
            kicker="METER PARAMETERS"
            title="Pulse & reverse settings"
            description="Configure pulse scaling and reverse-flow detection."
          />
          <div className="config-fields three-fields">
            <Field label="Reverse-flow threshold" hint="Register 0x000D">
              <Input
                type="number"
                min="0"
                max="65535"
                placeholder={
                  values[0x0d] === undefined
                    ? "Read current value"
                    : String(values[0x0d])
                }
                value={threshold}
                onChange={(e) => setThreshold(e.target.value)}
              />
              <Button
                variant="secondary"
                disabled={busy || !threshold}
                onClick={() => writeOne(0x0d, threshold, "Reverse threshold")}
              >
                Write
              </Button>
            </Field>
            <Field label="Base meter pulse" hint="1–10,000 L/pulse · 0x0010">
              <Input
                type="number"
                min="1"
                max="10000"
                value={basePulse}
                onChange={(e) => setBasePulse(e.target.value)}
              />
              <Button
                variant="secondary"
                disabled={busy || !basePulse}
                onClick={() =>
                  writeOne(0x10, basePulse, "Base pulse", 1, 10000)
                }
              >
                Write
              </Button>
            </Field>
            <Field label="Output pulse" hint="1–1,000 L/pulse · 0x0011">
              <Input
                type="number"
                min="1"
                max="1000"
                value={outputPulse}
                onChange={(e) => setOutputPulse(e.target.value)}
              />
              <Button
                variant="secondary"
                disabled={busy || !outputPulse}
                onClick={() =>
                  writeOne(0x11, outputPulse, "Output pulse", 1, 1000)
                }
              >
                Write
              </Button>
            </Field>
          </div>
          <div className="inline-info">
            <CircleHelp size={14} /> The output pulse equivalent should be
            greater than the base meter pulse equivalent.
          </div>
        </Card>
        <Card className="config-card">
          <CardTitle
            icon={<Waves size={17} />}
            kicker="PRESET COUNTERS"
            title="Flow baselines"
            description="Set the forward and reverse 32-bit baseline values (registers 0x0012–0x0015)."
          />
          <div className="config-fields two-fields">
            <Field label="Forward baseline" hint="Unsigned 32-bit counter">
              <Input
                type="number"
                min="0"
                max="4294967295"
                value={forwardBase}
                onChange={(e) => setForwardBase(e.target.value)}
              />
            </Field>
            <Field label="Reverse baseline" hint="Unsigned 32-bit counter">
              <Input
                type="number"
                min="0"
                max="4294967295"
                value={reverseBase}
                onChange={(e) => setReverseBase(e.target.value)}
              />
            </Field>
          </div>
          <div className="config-footer">
            <span>Writes four consecutive registers using function 0x10</span>
            <Button
              disabled={busy || !forwardBase || !reverseBase}
              onClick={writeBaselines}
            >
              <Save size={14} /> Write baselines
            </Button>
          </div>
          <div className="inline-warning">
            <CircleHelp size={14} /> The manual documents writable baseline
            registers, but no Modbus memory-clear/reset command. Writing zero
            changes these baselines only; it is not a documented meter-memory
            erase.
          </div>
        </Card>
        <Card className="config-card">
          <CardTitle
            icon={<Clock3 size={17} />}
            kicker="REAL-TIME CLOCK"
            title="Set device time"
            description="The meter stores year/month, day/hour, and minute/second in registers 0x0007–0x0009."
          />
          <div className="config-fields time-fields">
            <Field label="Date and time">
              <Input
                type="datetime-local"
                value={clockValue}
                onChange={(e) => setClockValue(e.target.value)}
              />
            </Field>
            <div className="clock-current">
              Current meter time
              <strong>
                {decodeClock([values[7], values[8], values[9]]) ??
                  "Read clock to view"}
              </strong>
            </div>
            <Button disabled={busy} onClick={writeClock}>
              <Save size={14} /> Set device time
            </Button>
          </div>
        </Card>
      </div>
      <aside className="config-aside">
        <Card className="aside-card">
          <div className="aside-title">
            <span className="icon-box blue-box">
              <Activity size={16} />
            </span>
            <strong>Read current settings</strong>
          </div>
          <p>Fetch all standard registers before making changes.</p>
          <Button variant="secondary" disabled={busy} onClick={readConfig}>
            <RefreshCw size={14} /> Read register map
          </Button>
          <div className="aside-status">
            <span
              className={
                Object.keys(values).length
                  ? "health-dot green"
                  : "health-dot neutral"
              }
            />
            {Object.keys(values).length
              ? `${Object.keys(values).length} values in memory`
              : "No values loaded"}
          </div>
        </Card>
        <Card className="aside-card">
          <div className="aside-title">
            <span className="icon-box amber-box">
              <ShieldCheck size={16} />
            </span>
            <strong>Write safety</strong>
          </div>
          <p>
            Meter writes are immediate. Read-only registers can still be
            accessed through the generic register browser if required by your
            device.
          </p>
          <div className="write-method">
            <code>0x06</code>
            <span>Single register</span>
          </div>
          <div className="write-method">
            <code>0x10</code>
            <span>Multiple registers</span>
          </div>
        </Card>
      </aside>
    </div>
  );
}

function CardTitle({
  icon,
  kicker,
  title,
  description,
}: {
  icon: React.ReactNode;
  kicker: string;
  title: string;
  description: string;
}) {
  return (
    <div className="config-title">
      <span className="config-title-icon">{icon}</span>
      <div>
        <span className="section-kicker">{kicker}</span>
        <h2>{title}</h2>
        <p>{description}</p>
      </div>
    </div>
  );
}

function RegisterBrowser({
  slave,
  busy,
  act,
  read,
  values,
  registers,
}: {
  slave: number;
  busy: boolean;
  act: (work: () => Promise<void>, success?: string) => Promise<void>;
  read: (address: number, count: number) => Promise<number[]>;
  values: Record<number, number>;
  registers: Register[];
}) {
  const [start, setStart] = useState("0");
  const [count, setCount] = useState("22");
  const [writeAddress, setWriteAddress] = useState("13");
  const [writeValue, setWriteValue] = useState("5");
  const [multiValues, setMultiValues] = useState("1, 1");
  const [filter, setFilter] = useState("");
  const [broadcast, setBroadcast] = useState(false);
  const filtered = useMemo(
    () =>
      registers.filter((item) =>
        `${item.name} ${item.group} ${hex(item.address)}`
          .toLowerCase()
          .includes(filter.toLowerCase()),
      ),
    [registers, filter],
  );
  const readRange = () =>
    act(async () => {
      const a = parseInt(start, 0);
      const n = Number(count);
      if (
        !Number.isInteger(a) ||
        a < 0 ||
        a > 0xffff ||
        !Number.isInteger(n) ||
        n < 1 ||
        n > 125
      )
        throw new Error("Enter an address 0..65535 and count 1..125");
      await read(a, n);
    }, "Register range read");
  return (
    <div className="register-page">
      <Card className="raw-card">
        <div className="raw-title">
          <div>
            <span className="section-kicker">FUNCTION 0x03</span>
            <h2>Read holding registers</h2>
            <p>Read any contiguous range from the selected meter.</p>
          </div>
          <span className="method-pill">
            0x03 <span>READ</span>
          </span>
        </div>
        <div className="raw-controls">
          <Field label="Start address">
            <Input
              type="number"
              min="0"
              max="65535"
              value={start}
              onChange={(e) => setStart(e.target.value)}
            />
          </Field>
          <Field label="Register count">
            <Input
              type="number"
              min="1"
              max="125"
              value={count}
              onChange={(e) => setCount(e.target.value)}
            />
          </Field>
          <Button disabled={busy} onClick={readRange}>
            <Search size={14} /> Read range
          </Button>
        </div>
      </Card>
      <Card className="table-card">
        <div className="table-heading">
          <div>
            <span className="section-kicker">REGISTER MAP</span>
            <h2>
              Mapped registers{" "}
              <span className="title-count">{registers.length}</span>
            </h2>
          </div>
          <div className="table-search">
            <Search size={15} />
            <Input
              placeholder="Filter registers…"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
            />
          </div>
        </div>
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>ADDRESS</th>
                <th>REGISTER</th>
                <th>GROUP</th>
                <th>ACCESS</th>
                <th>RAW VALUE</th>
                <th>HUMAN-READABLE</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((register) => (
                <tr key={register.address}>
                  <td>
                    <code>{hex(register.address)}</code>
                  </td>
                  <td>
                    <strong>{register.name}</strong>
                    {register.note && <small>{register.note}</small>}
                  </td>
                  <td>
                    <span className="group-pill">{register.group}</span>
                  </td>
                  <td>
                    <Badge
                      tone={
                        register.access === "read only" ? "neutral" : "blue"
                      }
                    >
                      {register.access}
                    </Badge>
                  </td>
                  <td className="value-cell">
                    {values[register.address] === undefined ? (
                      <span className="empty-value">—</span>
                    ) : (
                      <>
                        <code>{hex(values[register.address])}</code>
                        <span>{values[register.address]}</span>
                      </>
                    )}
                  </td>
                  <td className="decoded-cell">
                    {interpretRegister(register.address, values)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="table-foot">
          Register addresses use hexadecimal notation. Values are 16-bit
          unsigned words. Flow totals are converted as raw 32-bit counts ×
          register 0x0011 (L/pulse); baseline units are not specified by the
          manual.
        </div>
      </Card>
      <Card className="raw-card">
        <div className="raw-title">
          <div>
            <span className="section-kicker">FUNCTION 0x06</span>
            <h2>Write single register</h2>
            <p>
              Send one 16-bit value to the selected meter or broadcast address
              0.
            </p>
          </div>
          <span className="method-pill amber-pill">
            0x06 <span>WRITE</span>
          </span>
        </div>
        <div className="raw-controls">
          <Field label="Register address">
            <Input
              type="number"
              min="0"
              max="65535"
              value={writeAddress}
              onChange={(e) => setWriteAddress(e.target.value)}
            />
          </Field>
          <Field label="16-bit value">
            <Input
              type="number"
              min="0"
              max="65535"
              value={writeValue}
              onChange={(e) => setWriteValue(e.target.value)}
            />
          </Field>
          <Button
            disabled={busy}
            onClick={() =>
              act(
                async () => {
                  const address = Number(writeAddress);
                  const value = Number(writeValue);
                  const targetSlave = broadcast ? 0 : slave;
                  if (
                    ![address, value].every(Number.isInteger) ||
                    address < 0 ||
                    address > 65535 ||
                    value < 0 ||
                    value > 65535
                  )
                    throw new Error("Address and value must be 0..65535");
                  await api.post("/api/write-single", {
                    slave: targetSlave,
                    address,
                    value,
                  });
                  if (!broadcast) await read(address, 1);
                },
                broadcast
                  ? "Broadcast write sent (no reply expected)"
                  : "Single register write acknowledged",
              )
            }
          >
            Write register
          </Button>
        </div>
        <label className="broadcast-control">
          <input
            type="checkbox"
            checked={broadcast}
            onChange={(event) => setBroadcast(event.target.checked)}
          />
          <span>Broadcast write to address 0</span>
        </label>
      </Card>
      <Card className="raw-card">
        <div className="raw-title">
          <div>
            <span className="section-kicker">FUNCTION 0x10</span>
            <h2>Write multiple registers</h2>
            <p>
              Write a sequence of 16-bit words starting at the selected
              register.
            </p>
          </div>
          <span className="method-pill violet-pill">
            0x10 <span>WRITE</span>
          </span>
        </div>
        <div className="raw-controls">
          <Field label="Start address">
            <Input
              type="number"
              min="0"
              max="65535"
              value={writeAddress}
              onChange={(e) => setWriteAddress(e.target.value)}
            />
          </Field>
          <Field label="Values · comma separated">
            <Input
              value={multiValues}
              onChange={(e) => setMultiValues(e.target.value)}
              placeholder="e.g. 4660, 22136"
            />
          </Field>
          <Button
            disabled={busy}
            onClick={() =>
              act(
                async () => {
                  const address = Number(writeAddress);
                  const tokens = multiValues
                    .split(",")
                    .map((value) => value.trim());
                  const words = tokens.map(Number);
                  const targetSlave = broadcast ? 0 : slave;
                  if (
                    !Number.isInteger(address) ||
                    address < 0 ||
                    address > 65535 ||
                    !words.length ||
                    words.length > 123 ||
                    tokens.some((value) => !/^\d+$/.test(value)) ||
                    words.some(
                      (value) =>
                        !Number.isInteger(value) || value < 0 || value > 65535,
                    )
                  )
                    throw new Error(
                      "Enter an address 0..65535 and 1..123 unsigned 16-bit values",
                    );
                  await api.post("/api/write-multiple", {
                    slave: targetSlave,
                    address,
                    values: words,
                  });
                  if (!broadcast) await read(address, words.length);
                },
                broadcast
                  ? "Broadcast write sent (no reply expected)"
                  : "Multiple register write acknowledged",
              )
            }
          >
            Write registers
          </Button>
        </div>
        <label className="broadcast-control">
          <input
            type="checkbox"
            checked={broadcast}
            onChange={(event) => setBroadcast(event.target.checked)}
          />
          <span>Broadcast write to address 0 · no reply expected</span>
        </label>
        <div className="inline-warning">
          <AlertTriangle size={14} /> Raw writes bypass register access
          restrictions. Verify the address and values before sending.
        </div>
      </Card>
    </div>
  );
}

function interpretRegister(
  address: number,
  values: Record<number, number>,
): string {
  const value = values[address];
  const pulse = values[0x11];
  const flowGroups: Record<number, { label: string; high: number }> = {
    1: { label: "Total flow", high: 1 },
    2: { label: "Total flow", high: 1 },
    3: { label: "Forward flow", high: 3 },
    4: { label: "Forward flow", high: 3 },
    5: { label: "Reverse flow", high: 5 },
    6: { label: "Reverse flow", high: 5 },
  };
  const flow = flowGroups[address];
  if (flow) {
    if (address !== flow.high) return `Low word of ${flow.label.toLowerCase()}`;
    const count = combine32(values[flow.high], values[flow.high + 1]);
    if (count === null)
      return `Read both words to decode ${flow.label.toLowerCase()}`;
    const volume = formatVolume(count, pulse);
    return `${count.toLocaleString()} raw counts · ${volume ?? "read 0x0011 to convert"}`;
  }
  if (address === 0)
    return value === undefined ? "Not read" : `Slave address ${value}`;
  if (address >= 7 && address <= 9) {
    if (address === 7) {
      return (
        decodeClock([values[7], values[8], values[9]]) ??
        "Clock unavailable or invalid"
      );
    }
    return "Clock component · see decoded value at 0x0007";
  }
  if (address === 0x10 || address === 0x11) {
    return value === undefined ? "Not read" : `${value} L/pulse`;
  }
  if (address === 0x12 || address === 0x14) {
    const count = combine32(values[address], values[address + 1]);
    const direction = address === 0x12 ? "Forward" : "Reverse";
    return count === null
      ? `Read both words for ${direction.toLowerCase()} baseline`
      : `${direction} baseline · ${count.toLocaleString()} raw units`;
  }
  if (address === 0x13) return "Low word of forward baseline";
  if (address === 0x15) return "Low word of reverse baseline";
  if (address === 0x0d)
    return value === undefined
      ? "Not read"
      : "Threshold · unit not specified in manual";
  if (address === 0x0a)
    return value === undefined ? "Not read" : `Device type ID ${value}`;
  return value === undefined
    ? "Not read"
    : "Raw 16-bit value · unit not specified";
}

function Diagnostics({
  status,
  slave,
  setSlave,
  transactions,
  values,
  busy,
  act,
  refreshTransactions,
  read,
}: {
  status: Status | null;
  slave: number;
  setSlave: (value: number) => void;
  transactions: Transaction[];
  values: Record<number, number>;
  busy: boolean;
  act: (work: () => Promise<void>, success?: string) => Promise<void>;
  refreshTransactions: () => Promise<void>;
  read: (address: number, count: number) => Promise<number[]>;
}) {
  const debug = [values[0x19], values[0x1a], values[0x1b]];
  const [foundDevices, setFoundDevices] = useState<
    { slave: number; addressValue: number }[] | null
  >(null);
  const scanBus = () =>
    act(async () => {
      const result = await api.post<{
        devices: { slave: number; addressValue: number }[];
      }>("/api/scan", { start: 1, end: 247 });
      setFoundDevices(result.devices);
      if (result.devices.length === 1) setSlave(result.devices[0].slave);
    }, "Bus scan complete");
  return (
    <div className="diagnostics-page">
      <div className="diagnostics-grid">
        <Card className="diag-card">
          <div className="card-heading">
            <div>
              <span className="section-kicker">SERIAL LINK</span>
              <h2>Transport settings</h2>
            </div>
            <span className="icon-box blue-box">
              <Cable size={17} />
            </span>
          </div>
          <div className="diag-info">
            <div>
              <span>Serial port</span>
              <strong>{status?.serialPort ?? "—"}</strong>
            </div>
            <div>
              <span>Line format</span>
              <strong>
                {status
                  ? `${status.baudRate} baud · ${status.dataBits}${status.parity[0].toUpperCase()}${status.stopBits}`
                  : "—"}
              </strong>
            </div>
            <div>
              <span>Response timeout</span>
              <strong>{status?.timeoutMs ?? "—"} ms</strong>
            </div>
            <div>
              <span>Slave ID</span>
              <strong>{status?.defaultSlave ?? "—"} default</strong>
            </div>
          </div>
          <div className="diag-alert">
            <CircleHelp size={14} /> Serial link opens for each transaction. A
            disconnected device is reported on the next request.
          </div>
          <div className="scan-controls">
            <Button variant="secondary" disabled={busy} onClick={scanBus}>
              <Search size={14} /> Scan slave IDs
            </Button>
            <span>Checks 1–247 with short per-device probes.</span>
          </div>
          {foundDevices !== null && (
            <div className="scan-results">
              {foundDevices.length === 0 ? (
                <span className="scan-empty">
                  No device replied. Check meter power, A/B polarity, and baud
                  rate.
                </span>
              ) : (
                <>
                  <strong>
                    {foundDevices.length} meter
                    {foundDevices.length === 1 ? "" : "s"} found
                  </strong>
                  {foundDevices.map((device) => (
                    <Button
                      key={device.slave}
                      variant={device.slave === slave ? "primary" : "secondary"}
                      onClick={() => setSlave(device.slave)}
                    >
                      Slave {device.slave} · ID register{" "}
                      {hex(device.addressValue)}
                    </Button>
                  ))}
                </>
              )}
            </div>
          )}
        </Card>
        <Card className="diag-card">
          <div className="card-heading">
            <div>
              <span className="section-kicker">DEVICE INTERNALS</span>
              <h2>Debug registers</h2>
            </div>
            <span className="icon-box amber-box">
              <Activity size={17} />
            </span>
          </div>
          <p className="diag-description">
            Six internal debug bytes are exposed in register words
            0x0019–0x001B.
          </p>
          <div className="debug-grid">
            {debug.flatMap((word, index) =>
              [0, 1].map((byteIndex) => (
                <div key={`${index}-${byteIndex}`}>
                  <span>DEBUG {index * 2 + byteIndex + 1}</span>
                  <strong>
                    {word === undefined
                      ? "—"
                      : `0x${(byteIndex === 0 ? word >> 8 : word & 0xff).toString(16).toUpperCase().padStart(2, "0")}`}
                  </strong>
                </div>
              )),
            )}
          </div>
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() =>
              act(async () => {
                await read(0x19, 3);
              }, "Debug registers read")
            }
          >
            <RefreshCw size={14} /> Read debug values
          </Button>
        </Card>
      </div>
      <Card className="transactions-card">
        <div className="table-heading">
          <div>
            <span className="section-kicker">BUS ACTIVITY</span>
            <h2>
              Recent transactions{" "}
              <span className="title-count">{transactions.length}</span>
            </h2>
          </div>
          <Button variant="secondary" onClick={refreshTransactions}>
            <RefreshCw size={14} /> Refresh log
          </Button>
        </div>
        {transactions.length === 0 ? (
          <div className="empty-state">
            <div>
              <Activity size={20} />
            </div>
            <strong>No transactions yet</strong>
            <span>Modbus request and response frames will appear here.</span>
          </div>
        ) : (
          <div className="table-scroll">
            <table className="transactions-table">
              <thead>
                <tr>
                  <th>TIME</th>
                  <th>REQUEST FRAME</th>
                  <th>RESPONSE FRAME</th>
                  <th>LATENCY</th>
                  <th>RESULT</th>
                </tr>
              </thead>
              <tbody>
                {[...transactions].reverse().map((item, index) => (
                  <tr key={`${item.at}-${index}`}>
                    <td className="time-cell">
                      {new Date(item.at).toLocaleTimeString()}
                    </td>
                    <td>
                      <code>{formatHex(item.request)}</code>
                    </td>
                    <td>
                      <code>
                        {item.response ? (
                          formatHex(item.response)
                        ) : (
                          <span className="empty-value">—</span>
                        )}
                      </code>
                    </td>
                    <td>{item.durationMs} ms</td>
                    <td>
                      <Badge tone={item.error ? "red" : "green"}>
                        {item.error ? "Failed" : "OK"}
                      </Badge>
                      {item.error && (
                        <small className="tx-error">{item.error}</small>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="table-foot">
          Transaction history is held in memory and resets when the application
          restarts. CRC bytes are shown low byte first.
        </div>
      </Card>
      <div className="method-grid">
        <div>
          <code>0x03</code>
          <span>
            <strong>Read holding registers</strong>
            <small>1–125 contiguous words</small>
          </span>
        </div>
        <div>
          <code>0x06</code>
          <span>
            <strong>Write single register</strong>
            <small>One 16-bit register value</small>
          </span>
        </div>
        <div>
          <code>0x10</code>
          <span>
            <strong>Write multiple registers</strong>
            <small>1–123 contiguous words</small>
          </span>
        </div>
      </div>
    </div>
  );
}

function formatHex(frame: string) {
  return (
    frame
      .match(/.{1,2}/g)
      ?.join(" ")
      .toUpperCase() ?? frame
  );
}
function shortNumber(value: number) {
  return value >= 1_000_000
    ? `${(value / 1_000_000).toFixed(1)}M`
    : value >= 10_000
      ? `${(value / 1_000).toFixed(1)}K`
      : value.toLocaleString();
}
