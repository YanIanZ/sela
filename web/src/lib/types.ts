// Types mirroring the Go JSON views (internal/journey/views.go). Keep in sync
// with docs/API_SPEC.md.
export type Lang = 'id' | 'en';
export type Text = { id: string; en: string };
export type Provenance = 'live_meta' | 'recorded_demo' | 'manual' | 'test_double';

export type Segment = { id: string; text: string; origin: 'story' | 'clarification' };
export type Qualifiers = { negated: boolean; subject: 'self' | 'other'; time: string; certainty: string };
export type Kind = 'feeling' | 'experience' | 'impact' | 'duration' | 'context' | 'worry' | 'goal' | 'preference' | 'question';
export const KINDS: Kind[] = ['feeling', 'experience', 'impact', 'duration', 'context', 'worry', 'goal', 'preference', 'question'];
export type UnknownField = 'concern' | 'duration' | 'daily_impact' | 'goal' | 'support_preference';

export type Item = {
  id: string;
  kind: Kind;
  segmentId: string;
  quote: string;
  start: number;
  end: number;
  wording: string;
  wordingOrigin: 'model' | 'quote' | 'user';
  qualifiers: Qualifiers;
  status: 'unreviewed' | 'confirmed';
  provenance: Provenance;
  notes?: string[];
};

export type Consent = { granted: boolean; policyVersion: string; decidedAt: string; revision: number };

export type OpRecord = {
  id: string;
  kind: 'ledger_draft' | 'opening_draft';
  state: 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled';
  errorCode?: string;
  provenance?: Provenance;
  startedAt: string;
};

export type Preferences = { language: string; format: string; budget: string; availability: string; barrier: string; version: number };
export type OpeningLine = { text: string; basedOn: string[] };

export type ProjectedItem = { kind: Kind; text: string; quote?: string; qualifiers: Qualifiers };
export type Projection = {
  synthetic: boolean;
  language: Lang;
  briefVersion: number;
  items: ProjectedItem[];
  unknowns?: UnknownField[];
  opening?: string[];
  preferences?: Preferences;
  displayName?: string;
  ageRange?: string;
  preferredLanguage?: Lang;
  slotId: string;
  provenance: Provenance;
};

export type TimelineEvent = { kind: string; at: string; actor: string; detail?: string };
export type HandoffStatus = 'requested' | 'acknowledged' | 'clarification_requested' | 'declined' | 'completed';
export type ShareState = 'active' | 'revoked' | 'expired' | 'locked';

export type Handoff = {
  id: string;
  version: number;
  professionalId: string;
  slotId: string;
  briefVersion: number;
  fields: string[];
  projection: Projection;
  status: HandoffStatus;
  share: ShareState;
  shareReason?: string;
  createdAt: string;
  expiresAt: string;
  timeline: TimelineEvent[];
  clarifyReason?: string;
  declineReason?: string;
  planId?: string;
  replacesId?: string;
  opened: boolean;
  careReceived: false;
};

export type AIStatus = {
  provider: { mode: 'modal' | 'test_double' | 'disabled'; servedModel?: string; modelId?: string; revision?: string };
  available: boolean;
  likelyWarm: boolean;
  coldStartEstimateMs: number;
  inUse: number;
};

export type SessionView = {
  version: number;
  synthetic: true;
  language: Lang;
  identity: { mode: 'anonymous' | 'pseudonym'; pseudonym?: string };
  createdAt: string;
  expiresAt: string;
  idleExpiresAt: string;
  retention: 'session' | 'day';
  consents: Partial<Record<'ai_processing' | 'storage', Consent>>;
  policies: Record<'privacy' | 'ai_processing' | 'storage' | 'sharing', string>;
  safety: { level: 'none' | 'elevated' | 'crisis'; interrupted: boolean; thirdParty?: boolean };
  stage: string;
  story?: { version: number; mode: 'scenario' | 'practice'; scenarioId?: string; segments: Segment[] };
  ledger?: {
    version: number;
    items: Item[];
    unknowns: UnknownField[];
    skipped: UnknownField[];
    clarification?: UnknownField;
    provenance: Provenance;
    model?: string;
    approved?: { version: number; hash: string; at: string };
    reviewed: number;
    rejected?: Record<string, number>;
  };
  preferences?: Preferences;
  opening?: { lines: OpeningLine[]; provenance: Provenance; briefVersion: number; userEdited: boolean };
  handoffs: Handoff[];
  checkIns: { id: string; handoffId: string; optionId: string; at: string }[];
  ops: OpRecord[];
  ai: AIStatus;
  fixturesVersion: string;
};

export type Scenario = {
  id: string;
  version: string;
  kind: 'seeded' | 'crisis';
  language: 'id' | 'en' | 'mixed';
  title: Text;
  persona: Text;
  summary: Text;
  contentWarning?: Text | null;
  story: string;
};

export type Slot = { id: string; start: string; format: 'online' | 'in_person'; available: boolean };
export type Professional = {
  id: string;
  name: string;
  role: Text;
  category: string;
  languages: Lang[];
  formats: string[];
  costTier: string;
  costLabel: Text;
  availability: string[];
  area: Text;
  bio: Text;
  slots: Slot[];
  plans: string[];
};
export type Plan = { id: string; version: string; title: Text; steps: Text[]; attribution: Text };
export type Option = { id: string; label: Text; kind?: string };
export type NextStepDef = { id: string; title: Text; description: Text; action: string; barriers: string[] };
export type HelpResource = {
  id: string;
  name: string;
  region: string;
  regionName: Text;
  channels: { kind: 'phone' | 'whatsapp' | 'web'; value: string }[];
  hours: Text;
  cost: Text;
  description: Text;
  sources: { label: string; url: string; accessedOn: string }[];
  verificationStatus: string;
  lastVerified: string;
  verifiedBy: string;
  reverifyBy: string;
};
export type PolicyNotice = { version: string; title: Text; body: Text[] };

export type Catalog = {
  version: string;
  demoClock: string;
  synthetic: true;
  scenarios: Scenario[];
  professionals: Professional[];
  plans: Plan[];
  checkIns: Option[];
  clarifyReasons: Option[];
  declineReasons: Option[];
  nextSteps: NextStepDef[];
  clarifications: Record<UnknownField, Text>;
  help: HelpResource[];
  helpNotice: Text;
  policies: Record<'privacy' | 'ai_processing' | 'storage' | 'sharing', PolicyNotice>;
};

export type Reason = { criterion: string; match: boolean; value: string };
export type ProfessionalOption = { professionalId: string; reasons: Reason[]; slots: Slot[]; exact: boolean };
export type NextSteps = {
  preferences: Preferences;
  exact: ProfessionalOption[];
  near: ProfessionalOption[];
  steps: string[];
  handoffFirst: boolean;
  barrier: string;
};

export type RecipientHandoff = {
  id: string;
  version: number;
  professionalId: string;
  status: HandoffStatus;
  projection: Projection;
  expiresAt: string;
  timeline: TimelineEvent[];
  clarifyReason?: string;
  declineReason?: string;
  planId?: string;
  plans: string[];
  synthetic: true;
};

export type HandoffCreated = { handoff: Handoff; token: string; code: string; replay: boolean; session: SessionView };
