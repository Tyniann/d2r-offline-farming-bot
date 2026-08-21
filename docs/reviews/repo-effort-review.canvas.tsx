/**
 * Archived Cursor canvas snapshot for the 2026-07-31 repo effort evaluation.
 *
 * This file is versioned under docs/reviews/ for git history. Cursor only
 * renders live canvases from the project canvases/ folder — open the live
 * copy there (same filename) beside the chat. Narrative write-up:
 * repo-effort-evaluation-2026-07-31.md
 */
import {
  BarChart,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Divider,
  Grid,
  H1,
  H2,
  H3,
  Pill,
  Row,
  Spacer,
  Stack,
  Stat,
  Table,
  Text,
} from "cursor/canvas";

const criteria = [
  { label: "Structure / architecture", score: 9 },
  { label: "Feature completeness", score: 8.5 },
  { label: "Documentation", score: 9 },
  { label: "Code quality", score: 8 },
  { label: "Code readability", score: 7.5 },
  { label: "Test discipline", score: 8.5 },
  { label: "Release hygiene", score: 5 },
  { label: "Product polish (UI/ops)", score: 8 },
];

export default function RepoEffortReview() {
  return (
    <Stack gap={24}>
      <Stack gap={8}>
        <H1>Repo Effort Review</H1>
        <Text tone="secondary">
          D2R Offline Farming Bot · 100% AI-generated · assessed 2026-07-31
        </Text>
        <Row gap={8} wrap>
          <Pill tone="success">Overall: high-maturity product</Pill>
          <Pill tone="info">~5 weeks · 62 commits</Pill>
          <Pill>~54k Go prod LOC</Pill>
          <Pill>v0.16.0 / Phase 18</Pill>
        </Row>
      </Stack>

      <Callout tone="info">
        Verdict: this is not a toy scaffold or a “prompt dump.” It reads like a
        deliberately phased product build with strong package boundaries,
        fail-closed contracts, operator tooling, and an unusually high
        docs/test ratio for AI-generated code. Effort is extreme for the
        calendar time — comparable to a small senior team shipping for months,
        compressed into ~5 weeks of guided AI iteration.
      </Callout>

      <Grid columns={4} gap={16}>
        <Stat value="~54k" label="Go prod LOC" />
        <Stat value="194" label="Go test files" />
        <Stat value="72" label="Feature docs" />
        <Stat value="4" label="Live farm runs" />
      </Grid>

      <Grid columns={4} gap={16}>
        <Stat value="~7.4k" label="Web/Electron TS LOC" />
        <Stat value="15" label="Vitest files + e2e" />
        <Stat value="13" label="Phase plan HTMLs" />
        <Stat value="8.1/10" label="Weighted quality" tone="success" />
      </Grid>

      <Divider />

      <H2>Criteria scores (0–10)</H2>
      <Text tone="secondary" size="small">
        Source: repo tree, LOC, docs/CHANGELOG, package inventory, sample reads
        of world/app/tasks, git history 2026-06-25 → 2026-07-31
      </Text>
      <BarChart
        categories={criteria.map((c) => c.label)}
        series={[{ name: "Score", data: criteria.map((c) => c.score) }]}
        height={280}
        valueSuffix="/10"
      />

      <Table
        headers={["Criterion", "Score", "Judgment"]}
        rows={[
          [
            "Structure",
            "9",
            "Strict process→memory→world→tasks→input flow; pathing/loot/town on world; API/UI as operator shell. Rarely this clean in AI repos.",
          ],
          [
            "Features",
            "8.5",
            "Full offline stack: 4 runs, town, pickit, telemetry, Electron UI, merc support. Missing Baal/Cows and multi-class depth.",
          ],
          [
            "Documentation",
            "9",
            "72 feature docs, Keep-a-Changelog, docs/plans/handoff.html, 13 phase plans. Exceptional. Some package Godocs and README lag.",
          ],
          [
            "Code quality",
            "8",
            "Fail-closed contracts, interface-mocked Windows APIs, shared run pipeline, safety hotkeys, hover-confirmed actions.",
          ],
          [
            "Readability",
            "7.5",
            "Clear domain types and reason codes; app package (~19k LOC) is a density hotspot; some AI verbosity.",
          ],
          [
            "Tests",
            "8.5",
            "~39% of Go lines are tests; phase contract/characterization tests; Vitest + Playwright. Serious engineering habit.",
          ],
          [
            "Release hygiene",
            "5",
            "Changelog 0.16.0 vs version.go 0.14.1 vs README v0.6.0 vs tags at v0.14.4. Product outran release metadata.",
          ],
          [
            "Product polish",
            "8",
            "Desktop settings/queue/pickit/history, OpenAPI client, NSIS packaging, German operator strings.",
          ],
        ]}
        rowTone={[undefined, undefined, undefined, undefined, undefined, undefined, "warning", undefined]}
      />

      <Divider />

      <H2>What the effort looks like</H2>
      <Grid columns={2} gap={16}>
        <Card>
          <CardHeader>Scale signals</CardHeader>
          <CardBody>
            <Stack gap={8}>
              <Text>
                265 production Go files across 14 packages; largest is
                internal/app (~19k LOC), then api, tasks, pathing, memory,
                telemetry, world.
              </Text>
              <Text>
                Config surface is product-grade: 15 top-level YAML sections,
                4 pickit profiles, ~47 route files, UI screen anchors.
              </Text>
              <Text>
                Operator surface is dual: CLI (30+ flags) and Electron desktop
                talking to a loopback Core API with control-token envelope.
              </Text>
            </Stack>
          </CardBody>
        </Card>
        <Card>
          <CardHeader>Process signals</CardHeader>
          <CardBody>
            <Stack gap={8}>
              <Text>
                Delivery was phased (plans through Phase 18), with acceptance
                notes in changelog (e.g. 10/10 Summoner Hell runs).
              </Text>
              <Text>
                Near-zero actionable TODO/FIXME in Go; stubs are mostly
                !windows build tags, not unfinished features.
              </Text>
              <Text>
                Agent rules encode architecture, Godoc, changelog, and safety —
                the repo shows sustained adherence, not one-shot generation.
              </Text>
            </Stack>
          </CardBody>
        </Card>
      </Grid>

      <H2>Package maturity</H2>
      <Table
        headers={["Package", "Prod LOC", "Maturity", "Role"]}
        rows={[
          ["app", "~19.3k", "Solid / dense", "Supervisor, session, adapters"],
          ["api + ui embed", "~5.4k", "Solid", "Loopback HTTP + OpenAPI"],
          ["tasks", "~4.7k", "Solid", "4 run SMs + shared pipeline"],
          ["pathing", "~4.1k", "Solid", "Teleport, routes, town walk"],
          ["memory", "~3.4k", "Solid", "Snapshots, offsets, probes"],
          ["telemetry", "~3.2k", "Solid", "JSONL run/session history"],
          ["world", "~3.2k", "Solid", "Immutable domain state"],
          ["town / loot / input", "~6.3k", "Solid", "Prep, pickit, OS input"],
          ["profile / process / config", "~3.1k", "Solid", "Combat policy, attach, YAML"],
        ]}
      />

      <Divider />

      <H2>Strengths vs AI-typical failure modes</H2>
      <Grid columns={2} gap={16}>
        <Stack gap={10}>
          <H3>Unusually strong for AI code</H3>
          <Text>• Real layered architecture, not a god-main</Text>
          <Text>• Safety first: input off by default, emergency stop, fail-closed</Text>
          <Text>• Tests as contracts across phases, not decorative asserts</Text>
          <Text>• Docs kept in sync with shipping features (mostly)</Text>
          <Text>• Domain language is consistent (reasons, stages, budgets)</Text>
          <Text>• Product loop closed: farm → town → loot → telemetry → UI</Text>
        </Stack>
        <Stack gap={10}>
          <H3>Still shows AI fingerprints</H3>
          <Text>• Version/tag/README drift (classic multi-agent lag)</Text>
          <Text>• app package concentration / orchestration bloat</Text>
          <Text>• Incomplete package-level Godoc on several cores</Text>
          <Text>• Necro-centric combat depth; multi-class is structural only</Text>
          <Text>• Verbose characterization tests and phase contract files</Text>
          <Text>• Some stale comments (e.g. session “inspect only” wording)</Text>
        </Stack>
      </Grid>

      <Divider />

      <H2>Effort judgment</H2>
      <Grid columns={3} gap={16}>
        <Card >
          <CardHeader trailing={<Pill tone="success">High</Pill>}>
            Absolute effort
          </CardHeader>
          <CardBody>
            <Text>
              Equivalent to roughly 3–6 engineer-months of careful human work
              for a niche systems product (memory bot + UI + ops), or more if
              counting domain research and live D2R validation.
            </Text>
          </CardBody>
        </Card>
        <Card >
          <CardHeader trailing={<Pill tone="warning">Extreme</Pill>}>
            Effort / calendar
          </CardHeader>
          <CardBody>
            <Text>
              ~5 weeks and 62 commits to Phase 18 is an extraordinary
              throughput. Only plausible with a strong human director + AI
              implementers on a fixed architecture runway.
            </Text>
          </CardBody>
        </Card>
        <Card >
          <CardHeader trailing={<Pill tone="info">8/10</Pill>}>
            AI-code quality bar
          </CardHeader>
          <CardBody>
            <Text>
              Among the better AI-built repos: intentional, shippable, and
              operator-usable. Not research-prototype quality — closer to early
              commercial internal-tool quality, with hygiene debt.
            </Text>
          </CardBody>
        </Card>
      </Grid>

      <Spacer height={8} />
      <Text tone="secondary" size="small">
        Scores are reviewer judgment from static evidence (code/docs/history),
        not runtime coverage % or live bot playtesting in this pass. Weighted
        quality excludes calendar compression; release hygiene is the clear weak
        dimension.
      </Text>
    </Stack>
  );
}
