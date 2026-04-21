# Privacy Policy — Atma Satori

**Last updated:** April 21, 2026

This Privacy Policy explains how the Atma Satori mobile application ("the App", "we", "our") collects, uses, stores, and shares information. The App is published by **Ashumeet Samra** ("the Publisher").

Contact for privacy questions: **ashumeet.landmark@gmail.com**

---

## 1. Who we are

Atma Satori is a personal development reflection app that helps users record short statements ("discoveries") from seminar programs and delivers them back as random reflection reminders throughout the day. Access is limited to users who hold a valid invitation code.

---

## 2. Information we collect

When you use the App, we collect only the information necessary to operate it:

- **Identity information you provide**
  - Your **name**
  - Your **email address**
  - The **invitation code** you were issued
- **Automatically collected identifiers**
  - A per-installation **device identifier** (Apple's `identifierForVendor` on iOS, or the analogous platform identifier on Android). This is not a hardware serial number and cannot be used to identify you across unrelated apps or by other developers.
- **Content you create inside the App**
  - The short reflection statements ("discoveries" or "bullets") you write
  - The sessions that group them
  - Notification schedules and related preferences
- **Operational metadata**
  - Timestamps of account registration and updates

We do **not** collect: precise location, contacts, photos, microphone data, advertising identifiers, browsing history, or payment information.

---

## 3. How we use your information

We use the information collected to:

- Create and maintain your account and validate your invitation code
- Save your discoveries and sessions locally on your device
- Schedule and deliver local reflection notifications
- Send the content of a discovery to an AI analysis service **only when you explicitly request analysis** inside the App
- Enforce a one-active-device rule (re-registering on a new device automatically signs out the prior device)

We do not use your information for advertising, profiling, or any form of automated decision-making that produces legal or similarly significant effects.

---

## 4. Third parties we share data with

We share the minimum information required with the following service providers, each of which processes the data on our behalf:

- **Amazon Web Services (AWS)** — hosting provider. Stores your account record (name, email, device identifier, invitation code, timestamps) in Amazon DynamoDB and routes AI analysis requests through AWS Lambda. Region: US West (Oregon). See https://aws.amazon.com/privacy/.
- **Google LLC (Gemini API)** — AI provider. When you explicitly request analysis of a discovery or session, the text of those discoveries is transmitted to Google's Gemini API and returned as analysis. Use of this API is subject to Google's terms at https://ai.google.dev/terms. We do not send your name, email, or device identifier to Google.

We do **not** sell your personal information, and we do **not** share it with advertisers, data brokers, or analytics providers.

---

## 5. Data storage and retention

- **On your device:** discoveries, sessions, notification schedules, and your locally-cached name, email, and invitation code are stored in the App's local database (SwiftData/SQLite). This data stays on your device until you sign out or delete the App.
- **On our servers (AWS DynamoDB):** your account record (name, email, device identifier, invitation code, timestamps) is retained for as long as your account remains active.
- **AI requests:** the text of your discoveries is sent to Google's Gemini API per-request. We do not persist the content of those requests on our servers. Google's handling of API content is governed by its own terms.

---

## 6. Permissions we request (Android)

The Android version of the App requests the following permissions:

- **`POST_NOTIFICATIONS`** — required to deliver scheduled reflection reminders.
- **`INTERNET`** / **`ACCESS_NETWORK_STATE`** — required to communicate with our API for registration and AI analysis requests.
- **`SCHEDULE_EXACT_ALARM`** / **`USE_EXACT_ALARM`** (if applicable) — required to fire reflection notifications at the scheduled times you configure.

The App does **not** request or use the Advertising ID (`AD_ID`) permission.

---

## 7. Your rights and choices

You can:

- **View or correct your information** by re-registering inside the App (re-registration overwrites your name, email, and device record).
- **Sign out at any time** via the in-app Settings menu. Signing out clears all local data on your device.
- **Request deletion of your server-side account record** by emailing **ashumeet.landmark@gmail.com** from the email address associated with your account. We will delete your record from our database within 30 days of a verified request.
- **Switch devices** simply by re-registering on the new device. The prior device will be automatically signed out on its next network call.

Depending on where you live, you may have additional rights under applicable laws such as the GDPR, UK GDPR, or the CCPA (including the rights to access, portability, correction, deletion, and to object to processing). You can exercise any of these rights by contacting us at the address above.

---

## 8. Security

We use industry-standard protections to safeguard your information:

- All communication between the App and our API uses HTTPS/TLS in transit.
- Data stored in AWS DynamoDB benefits from AWS-managed encryption at rest.
- Secret keys (such as the AI service key) are stored server-side in AWS Systems Manager Parameter Store as SecureString values and are never sent to the App.

No method of transmission or storage is 100% secure, and we cannot guarantee absolute security.

---

## 9. Children's privacy

The App is not directed to children under 13 (or under 16 in jurisdictions where that higher age applies, such as the European Economic Area and the United Kingdom). We do not knowingly collect personal information from children. If you believe a child has provided us with personal information, please contact us and we will delete it.

---

## 10. International users

The App is operated from the United States and relies on AWS infrastructure in the United States (us-west-2). If you access the App from outside the United States, your information will be transferred to, stored in, and processed in the United States.

---

## 11. Changes to this policy

We may update this Privacy Policy from time to time. When we do, we will revise the "Last updated" date at the top of this page. Material changes will be communicated through the App or via the email address associated with your account.

---

## 12. Contact

For any questions about this Privacy Policy or our handling of your information, please contact:

**Ashumeet Samra**
Email: **ashumeet.landmark@gmail.com**
