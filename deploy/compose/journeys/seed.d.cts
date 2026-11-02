// Types for seed.mjs, the helper the stack journeys use. See seed.mjs for what each call does.
export const ONBOARDING: string;
export const BACKOFFICE: string;
export const WORKSPACE: string;

export interface Company {
  onboardingId: string;
  companyPartyId: string;
  personPartyIds: string[];
}

export interface OnboardingStatus {
  onboardingId: string;
  state: string;
  companyPartyId?: string;
  personPartyIds?: string[];
  accountIds?: string[];
  steps?: string[];
  declineReason?: string;
}

export interface Review {
  review_id: string;
  onboarding_id: string;
  subject_party_id: string;
  status: string;
  decision: string;
}

export function stackSecret(): string;
export function nationalId(): string;
export function startCompany(label: string): Promise<Company>;
export function verdict(onboardingId: string, subjectPartyId: string, value: "APPROVED" | "DECLINED" | "REVIEW"): Promise<void>;
export function onboardingState(onboardingId: string): Promise<OnboardingStatus>;
export function waitForState(onboardingId: string, state: string, timeoutMs?: number): Promise<OnboardingStatus>;
export function waitForReview(onboardingId: string, subjectPartyId: string, timeoutMs?: number): Promise<Review>;
export function decideReview(reviewId: string, decision: "APPROVED" | "DECLINED", note: string): Promise<Review>;
export function waitForAccounts(partyId: string, timeoutMs?: number): Promise<{ account_id: string; asset: string }[]>;
