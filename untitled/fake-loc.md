# Route Mock
## Platform
Android (Kotlin/Java)

## Primary Goal
- To streamline the testing of location-dependent features by allowing QA engineers and Developers to mock single points and simulate moving routes via JSON Input.

## 1. Product Overview
Route Mock is a developer utility tool designed to override the Android system's GPS signals. Unlike standard mock location apps that focus on static spoofing, Route Mock prioritizes "Journey Testing"—the ability to reproduce complex travel paths using pre-defined data sets.

### Target Audience
- QA Engineers: Validating distance tracking, geofencing, and ride-hailing logic.
- Android Developers: Debugging location-services code without leaving their desk.


## 2. Core Feature Specifications

### Feature Set 1: Single Location Mocking

- Objective: Allow the user to teleport the device to a static specific coordinate instantly.

#### Functional Requirements:
##### Map Interface:
- The home screen must render a map using **OpenStreetMap (OSM)** (e.g., via osmdroid).
- **Long Press Action**: Long-pressing anywhere on the map drops a pin and updates the target coordinates.

##### Location Search:
- **Search Bar**: Integration with a free geocoding API (e.g., **Nominatim**).
  - Implementation Note: **Debounce** input to respect API rate limits (1 request/sec).
- Users can type an address or POI (e.g., "Monas, Jakarta").
- Selection moves the map camera and pin to that location.

##### Activation:
- **"Set Location" Button**: A floating action button (FAB) or persistent bottom button.
- **Action**: When clicked, the app injects the mock location into the Android OS (Latitude/Longitude).
- **Visual Feedback**: A persistent notification indicates "Route Mock is active" (required by Android to keep the service alive).

### Feature Set 2: Route Simulation
- **Objective**: Simulate movement along a path to test app behavior during travel.

#### Functional Requirements:

##### Route Upload
- **User can select a .json file** from the device storage.
- **JSON Parser Logic**: The app must automatically detect and parse two specific schemas:
  - **Schema A (Object-wrapped)**: `{ "data": [ ... ] }`
  - **Schema B (Array-root)**: `[ ... ]`

##### Route Management
- **Naming**: Upon successful parsing, prompt the user to enter a "Route Name" (e.g., "Home to Office Test").
- **Local Storage**: Save the parsed route and the custom name to a local database (Room/SQLite).
- **Saved List**: A generic list view displaying all saved routes.

##### Route Playback
- **Play/Stop Controls**: Users can select a saved route and click "Play".
- **Simulation Logic (Smooth Drive)**:
  - The app must calculate intermediate points between coordinates using **Linear Interpolation** (straight line) to simulate smooth movement based on the selected speed.
  - **End-of-Route Behavior**: **Hold**. When the route finishes, the app continues broadcasting the last known coordinate until explicitly stopped.
- **Playback Settings (UI Options)**:
  - **Speed**:
    - User can set a target speed in **km/h**.
    - Option for **Random Speed**: User defines **Min/Max** range (e.g., 20km/h - 40km/h) to simulate traffic.
  - **Accuracy**:
    - User can set a fixed accuracy radius (e.g., 5m).
    - Option for **Noise/Random Accuracy**: User defines **Min/Max** range (e.g., 5m - 20m) to simulate realistic GPS fluctuation.
  - **Heading/Bearing**:
    - **Automatic**: Calculate bearing based on the next coordinate.
    - **Manual**: User can force a specific static heading.

## 3. Technical & Data Requirements

### 3.1. JSON Parsing Logic
The importer must validate the JSON. If the format is incorrect, an error toast must appear.

**Supported Format A:**
```json
{
  "data": [
    { "latitude": -6.2619, "longitude": 106.8657 },
    { "latitude": -6.257751, "longitude": 106.87 }
  ]
}
```

**Supported Format B:**
```json
[
  { "latitude": -6.2619, "longitude": 106.8657 },
  { "latitude": -6.257751, "longitude": 106.87 }
]
```

### 3.2. Android System Permissions
The app will require the following manifest permissions:
- `android.permission.ACCESS_FINE_LOCATION`
- `android.permission.ACCESS_COARSE_LOCATION`
- `android.permission.INTERNET` (For OpenStreetMap & Nominatim)
- `android.permission.FOREGROUND_SERVICE`
- `android.permission.FOREGROUND_SERVICE_LOCATION` (Required for Android 14+)

**Pre-requisite**: The user must manually enable "Developer Options" on their phone and select Route Mock as the "Mock Location App."

## 4. User Flow & UI Design

### Navigation Architecture
The app defines two distinct execution modes, accessed via a **Bottom Navigation Bar**:
1.  **Routes (Home)**: Library management and execution of complex paths.
2.  **Teleport (Map)**: Instant single-point injection.
*Global State Rule*: If a simulation is active, the Bottom Nav displays a pulsed "Active" indicator.

### Screen 1: Routes Dashboard (Home)
**UI Layout:**
-   **Header**: Title "My Journeys".
-   **Content**:
    -   **Empty State**: Winding road illustration + "Import Route" CTA.
    -   **List View**: Cards displaying:
        -   **Route Name** & Distance/Duration.
        -   **Play Button**: Primary action on the right.
        -   **Context Menu**: Edit Name, Share JSON, Delete.
-   **FAB**: "+" to Import JSON.

**User Flow (Route Simulation):**
1.  **Setup**: Tap Play -> generic "Config Sheet" appears (Speed, Accuracy).
2.  **Action**: Tap "Start Simulation".
3.  **Live State (The Monitor)**:
    -   App **stays open** and switches to a Map View tracking the moving pin.
    -   **HUD Overlay**: Shows "Current Speed", "Distance Remaining".
    -   **Controls**: "Pause", "Stop", and "Minimize" (User chooses to leave).
    -   **Backgrounding**: If user leaves, persistent notification takes over.

### Screen 2: Teleport Map
**UI Layout:**
-   **Full-Screen Map**: OSM-based.
-   **Interaction**:
    -   **State A (Idle)**: Search bar + "Teleport Here" FAB.
    -   **State B (Conflict)**: If Route Simulation is running, this screen is in "Read-Only" mode tracking the route, with a button "Stop Current Route to Teleport".

### Onboarding & Permissions (First Run)
**Goal**: Reduce the "Developer Options" friction.
1.  **Detection**: If permissions missing, launch **Guided Setup**.
2.  **Educational Overlay**: Show a looping GIF/Animation of *exactly* where the "Mock Location App" setting is in the Android System menu.
3.  **Action**: Button "Open Developer Options" (Deep link if possible, or generic Settings).

## 5. Potential Future Improvements (Backlog)
- **Looping**: A toggle to repeat the route indefinitely.
- **GPX Support**: Support standard GPX file formats alongside JSON.