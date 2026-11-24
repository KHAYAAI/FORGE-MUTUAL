# Regulatory return forms: where they come from

Read 2026-11-24. `scripts/fetch-return-forms.sh` downloads every file below and fails if its SHA-256 has changed, which
means the Prudential Authority (PA) has published a new version and the line mapping must be re-read.

## What changed in the law

- **Banks (BA returns).** The Minister removed every BA return and its completion instructions from the Regulations relating to
  Banks with effect from 1 February 2025. The PA now prescribes them in **Directive 1 of 2025** (ref. 15/8/1/3, dated 29 January 2025),
  with each form and its instructions as an annexure (50 enclosures: Annexure 1A is the list of returns, frequencies and deadlines).
- **Mutual banks (DI returns).** Mutual Banks **Directive 1 of 2025** (ref. 15/8/2/3, dated 13 August 2025) gives notice that every DI
  return and its instructions are being removed from the Regulations relating to Mutual Banks and will be issued as PA directives. The PA
  then published a **proposed directive** (dated 13 August 2025, comments by 26 September 2025, proposed start 1 December 2025) carrying
  every DI form and instruction as annexures. Paragraph 1.5.3 of the directive says the forms themselves are not changed by the move.
  A law-firm note and the consolidated regulations indicate the deletion took effect through GN 7414 of 29 April 2026; **whether the
  proposed directive has been issued in final form was not confirmed** (VERIFY with the lawyer or the PA).
- The DI forms in the proposed directive are scans of the Government Gazette pages (DI 100: Gazette 22738, 5 October 2001; DI 200:
  Gazette 15381, 28 December 1993). The line numbers and labels in `services/bank/reg-reporting/internal/returns/forms/` were transcribed
  from those scans by reading them, not by optical character recognition. **A second person must check the transcription against the
  scans** before any return is filed.

## Files

| File | Publisher, date | SHA-256 |
|---|---|---|
| [Annexures to the proposed directive (DI forms)](https://www.resbank.co.za/content/dam/sarb/publications/prudential-authority/pa-documents-issued-for-consultation/2025/proposed-directive-for-mutual-banks-on-di-returns/Annexures%20to%20the%20proposed%20directive.zip) | PA, 2025 | `146d62dab990ad45c449bdde3d9f90ffff4c48efcf249130d41504c9944e82b5` |
| [Proposed Directive for Mutual banks on DI Returns](https://www.resbank.co.za/en/home/publications/publication-detail-pages/prudential-authority/pa-documents-issued-for-consultation/2025/Proposed-Directive-for-Mutual-banks-on-DI-Returns) | PA, 13 August 2025 | `e9ddb08cde133c2eb2ab5a1fbe64959cb13789015c974a1cb985db870fb52b70` |
| [Mutual Banks Directive 1 of 2025](https://www.resbank.co.za/en/home/publications/publication-detail-pages/prudential-authority/pa-deposit-takers/Mutual-Banks-Directive/2025/D1-2025-Mutual-banks) | PA, 13 August 2025 | `8a2513bbdfbe2ae4635ebfd9f02989a430f28079fed6ad4c0d293af37a9ec821` |
| [Banks Directive 1 of 2025](https://www.resbank.co.za/en/home/publications/publication-detail-pages/prudential-authority/pa-deposit-takers/banks-directives/2025/Directive-returns-to-be-submitted-to-the-PA) | PA, 29 January 2025 | `fe671a0e3c3211cc6e6a48481429e79f5d4fc9bde9b88dc49f781102de16a081` |
| Banks Directive 1 of 2025, Returns - Annexures.zip (same page) | PA, 2025 | `58e1c9e03c390a6e6ff425012a9befd84b9e74b7be59a7226ae2f14c868a7231` |

Also published and not yet used: the PA's [Banks Returns (Umoja)](https://www.resbank.co.za/en/home/what-we-do/Prudentialregulation/banks-returns-umoja)
page, with per-form instruction bundles and a submission schedule, and a proposed directive for BA returns from 1 July 2026.

## Catalogue

[`catalogue.csv`](catalogue.csv) lists all 27 DI forms and 27 BA forms with the title, frequency, deadline and source annexure, and how Forge
treats each: **built**, **to-build**, **nil** (the form applies but Forge has nothing to report, and the code checks that), or
**not-applicable** (the form is marked N/A for a mutual bank in the Republic, or Forge has no foreign operations).
