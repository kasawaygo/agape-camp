# AGAPE CAMP & TRES DIAS

**Event Registration & Management Systems**

Two web-based registration systems developed for real church events in South Korea.

The projects focus on backend development with Go, including HTTP request handling, PostgreSQL integration, form validation, and administrative tools.

## Tech Stack

- **Backend:** Go, net/http, JSON
- **Database:** PostgreSQL, pgx
- **Frontend:** HTML, CSS, JavaScript
- **Authentication:** Session-based admin authentication
- **Deployment:** Render, Supabase

## Projects

### 1. AGAPE CAMP

A registration management system for a youth camp.

**Features:**
- Participant and volunteer registration
- Dynamic forms based on registration role
- Input validation
- Allergy and contact information collection
- PostgreSQL database integration
- Admin dashboard with search and filters
- Registration management and Excel export

### 2. TRES DIAS

A separate registration system for a church event, built within the same Go application.

**Features:**
- Participant and servant registration forms
- Role-specific registration fields
- Backend form processing
- Database integration
- Administrative registration management

## Project Structure

```text
agape-camp/
├── main.go
├── tresdias.go
├── go.mod
├── go.sum
├── web/
│   ├── admin/
│   └── tres-dias/
├── images/
├── render.yaml
└── *.sql
```

## Development Status

Both systems are being actively developed and improved.

Current areas of focus include:
- Learning more about PostgreSQL and SQL
- Improving backend code organization
- Testing and error handling
- Deployment and maintenance
- Learning Git and development workflows

## About the Developer

**Konstantin Chzhen**

Computer Science student at Dongseo University, Busan, South Korea.

I'm currently learning backend development with Go and building practical projects to improve my understanding of HTTP APIs, databases, and web application development.

I'm interested in junior backend developer and internship opportunities where I can contribute to real projects and learn from experienced engineers.

**GitHub:** https://github.com/kasawaygo
