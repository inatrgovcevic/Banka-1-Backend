-- liquibase formatted sql

-- changeset client-service:10 context:dev
-- DEV-ONLY: promote sve mock klijente na CLIENT_TRADING tako da mogu da testiraju
-- berzu (Celina 3) i OTC (Celina 4). Bez ove izmene /orders/buy i /orders/sell
-- vracaju 403 jer @PreAuthorize("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')")
-- ne prihvata CLIENT_BASIC. Production deploy mora ovo da preskoci (context=prod).
UPDATE clients SET role = 'CLIENT_TRADING' WHERE role = 'CLIENT_BASIC';
