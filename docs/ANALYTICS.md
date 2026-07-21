# Analytics specification

## 1. Safety boundary

Analytics describes recorded data. It does not diagnose, identify a medical cause, predict an emergency or recommend starting/stopping medication.

LLM is not allowed to calculate metrics. Optional natural-language summaries receive already calculated aggregates and must preserve qualifiers and sample sizes.

## 2. Eligible data

Default filter for every metric:

```sql
status = 'confirmed' AND deleted_at IS NULL
```

Additionally:

- use the user timezone when assigning events to calendar days;
- exclude pending/rejected/superseded events;
- distinguish unknown end from zero duration;
- distinguish “no entry” from explicit “no headache”;
- expose coverage and missingness next to results;
- record analytics formula/version in response metadata.

## 3. Coverage metrics

- observation window in days;
- diary days with any data;
- days with explicit check-in;
- days with explicit no-headache check-in;
- number of confirmed/pending entries;
- number and percentage of closed pain episodes;
- percentage of medication events with known dose and later effect observation.

Coverage must be shown before correlations.

## 4. Headache metrics

- `headache_days`: distinct local dates intersected by a confirmed headache episode/observation;
- `confirmed_headache_free_days`: explicit no-headache days only;
- episode count;
- open/closed episode count;
- duration median/p25/p75 for closed episodes;
- intensity average/max and distribution;
- time-of-day distribution of episode start;
- day-of-week distribution;
- location/laterality/quality/symptom frequencies;
- functional-impact distribution;
- rolling 7/30/60/90-day views.

An episode crossing midnight contributes to both headache days but only one episode.

## 5. Medication metrics

- intake count and distinct intake days by normalized medication;
- unknown dose count;
- linked headache episodes;
- time from pain start to intake;
- recorded effect rating;
- intensity change in configured 30/60/120-minute windows when observations exist;
- repeat intake interval.

Never interpret missing follow-up as “did not work”. Display `recorded_effect_n` separately.

Medication-overuse indicators are post-MVP and require medication-class normalization plus reviewed wording. NICE guidance is a source for review, not an automatic treatment instruction: https://www.nice.org.uk/guidance/cg150/chapter/recommendations

## 6. Other domains

### Sleep

- duration/quality by day;
- comparison of sleep before headache start vs comparable non-headache days;
- missing sleep days.

### Activity

- minutes and intensity by day;
- activity within configurable windows before episode start;
- activity avoided/functional impact only if explicitly reported.

### Wellbeing

- wellbeing, stress, mood and energy rolling series;
- day-before/day-of headache comparison;
- do not interpolate missing scores.

### Food/drink/measurements

Initially show timelines/counts. Association analysis is enabled only for sufficiently repeated normalized categories.

## 7. Association analysis

Goal: identify hypotheses worth observing, not causes.

For each normalized exposure (for example short sleep, caffeine, high stress, intense activity):

1. Define a versioned exposure rule and pre-headache window.
2. Count headache starts preceded by exposure.
3. Count comparable control windows with and without exposure.
4. Report raw counts, baseline rate, effect estimate and uncertainty where meaningful.
5. Apply minimum gates.
6. Label result `possible_association`, never `trigger` or `cause`.

Initial implementation may use transparent 2x2 counts and risk ratio with confidence interval. More advanced case-crossover modeling is a later version and must be documented with tests.

## 8. Minimum gates

Basic summaries can appear immediately. Association cards require all of:

- observation window >= 56 days;
- >= 8 headache starts;
- exposure recorded on >= 10 days/windows;
- >= 5 exposed and >= 5 unexposed comparable windows;
- missingness for required field <= 50%;
- result includes exact sample counts.

These are conservative product defaults, configurable only through a versioned analytics rule change—not per-request UI tweaking.

If gates fail, return a reason such as:

```json
{
  "status": "insufficient_data",
  "requirements": {
    "observation_days": {"actual": 31, "required": 56},
    "headache_starts": {"actual": 4, "required": 8}
  }
}
```

## 9. Calendar aggregation

Calendar response is calculated per local date and mode. Icon/color thresholds must be documented and stable:

- pain: none / mild 1–3 / moderate 4–6 / severe 7–10 / unknown intensity;
- medication: intake count, not “safe/unsafe” color;
- activity: recorded duration tiers;
- sleep/wellbeing: value plus unknown state.

Never color a day green merely because no entry exists.

## 10. Doctor/export report

Post-MVP printable report should contain:

- period and timezone;
- data coverage;
- headache days/episodes/duration/intensity;
- associated symptoms and functional impact;
- medication intake days and recorded response;
- possible associations with sample sizes;
- event timeline appendix;
- disclaimer that data is user-recorded and not a diagnosis.

NICE recommends capturing diary data for at least 8 weeks; use 8 weeks as the default report window: https://www.nice.org.uk/guidance/cg150/chapter/recommendations

## 11. Testing analytics

Every metric must have fixture tests covering:

- timezone boundary and daylight-saving timezone even if default has no DST;
- episode crossing midnight;
- open episode;
- deleted/superseded/pending event;
- missing intensity/dose/effect;
- duplicate observation;
- inclusive/exclusive range endpoints;
- zero denominator;
- insufficient-data gates;
- stable formula version.
