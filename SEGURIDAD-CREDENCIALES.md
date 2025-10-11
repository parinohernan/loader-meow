# 🔐 Seguridad y Manejo de Credenciales

## ⚠️ IMPORTANTE: Credenciales Sensibles

Este proyecto utiliza archivos de configuración con credenciales sensibles que **NUNCA** deben ser subidos al repositorio Git.

## 📁 Archivos Sensibles

Los siguientes archivos están incluidos en `.gitignore` y contienen información sensible:

- `mysql-config.env` - Credenciales de MySQL
- `run-dev.bat` - Script con credenciales embebidas (Windows)
- `store/` - Base de datos local con sesión de WhatsApp

## ✅ Archivos Seguros (Incluidos en Git)

Estos archivos SÍ deben estar en el repositorio:

- `mysql-config.env.example` - Plantilla sin credenciales
- `run-dev.bat.example` - Plantilla de script sin credenciales
- `.gitignore` - Lista de archivos a ignorar

## 🚀 Configuración Inicial

### Windows

1. **Copia el archivo de ejemplo:**

   ```bash
   copy run-dev.bat.example run-dev.bat
   ```

2. **Edita `run-dev.bat` con tus credenciales:**

   ```batch
   set DB_HOST=tu_servidor
   set DB_PORT=3306
   set DB_USER=tu_usuario
   set DB_PASSWORD=tu_password_real
   set DB_NAME=caricaloader
   ```

3. **Ejecuta:**
   ```bash
   ./run-dev.bat
   ```

### macOS/Linux

1. **Copia el archivo de ejemplo:**

   ```bash
   cp mysql-config.env.example mysql-config.env
   ```

2. **Edita `mysql-config.env` con tus credenciales:**

   ```env
   DB_HOST=tu_servidor
   DB_PORT=3306
   DB_USER=tu_usuario
   DB_PASSWORD=tu_password_real
   DB_NAME=caricaloader
   ```

3. **Ejecuta:**
   ```bash
   export $(cat mysql-config.env | xargs) && wails dev
   ```

## 🔍 Verificar que NO Subes Credenciales

Antes de hacer commit, verifica:

```bash
git status
```

**NO deben aparecer:**

- `mysql-config.env`
- `run-dev.bat`
- `store/`

**SÍ deben aparecer:**

- `mysql-config.env.example`
- `run-dev.bat.example`

## 🛡️ Mejores Prácticas

### 1. Variables de Entorno en Producción

Para producción, usa variables de entorno del sistema:

**Windows:**

```cmd
setx DB_HOST "tu_servidor"
setx DB_USER "tu_usuario"
setx DB_PASSWORD "tu_password"
setx DB_NAME "caricaloader"
```

**Linux/macOS:**

```bash
export DB_HOST="tu_servidor"
export DB_USER="tu_usuario"
export DB_PASSWORD="tu_password"
export DB_NAME="caricaloader"
```

### 2. Archivo .env (Alternativa)

Puedes crear un archivo `.env` (también en `.gitignore`):

```env
DB_HOST=tu_servidor
DB_PORT=3306
DB_USER=tu_usuario
DB_PASSWORD=tu_password
DB_NAME=caricaloader
```

### 3. Gestores de Secretos

Para entornos empresariales, considera:

- **HashiCorp Vault**
- **AWS Secrets Manager**
- **Azure Key Vault**
- **Google Secret Manager**

## ⚠️ Si Subiste Credenciales Accidentalmente

### 1. Elimina el archivo del historial de Git

```bash
# Eliminar archivo del historial
git filter-branch --force --index-filter \
  "git rm --cached --ignore-unmatch mysql-config.env" \
  --prune-empty --tag-name-filter cat -- --all

# Forzar push (¡CUIDADO!)
git push origin --force --all
```

### 2. Cambia TODAS las credenciales comprometidas

- Cambia la contraseña de MySQL
- Revoca accesos comprometidos
- Actualiza todas las instancias

### 3. Habilita autenticación de dos factores

En tu servidor MySQL:

- Configura 2FA si está disponible
- Limita acceso por IP
- Usa certificados SSL

## 📊 Checklist de Seguridad

Antes de cada commit:

- [ ] Verificar que `mysql-config.env` NO está staged
- [ ] Verificar que `run-dev.bat` NO está staged
- [ ] Verificar que `store/` NO está staged
- [ ] Archivos `.example` SÍ están staged
- [ ] `.gitignore` está actualizado
- [ ] README tiene instrucciones claras

## 🔗 Conexión Remota Segura

Si conectas a un servidor MySQL remoto:

### 1. Usa SSL/TLS

Actualiza la cadena de conexión en `config.go`:

```go
return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=true&loc=Local&tls=true",
    c.User, c.Password, c.Host, c.Port, c.Database, c.Charset)
```

### 2. Configura Firewall

Solo permite conexiones desde IPs conocidas:

```sql
CREATE USER 'admin_remoto'@'tu_ip' IDENTIFIED BY 'password';
GRANT ALL PRIVILEGES ON caricaloader.* TO 'admin_remoto'@'tu_ip';
```

### 3. Usa SSH Tunnel

Para mayor seguridad:

```bash
ssh -L 3306:localhost:3306 usuario@servidor
```

Luego conecta a `localhost:3306`

## 📝 Documentación Adicional

- [MySQL Security Best Practices](https://dev.mysql.com/doc/refman/8.0/en/security-guidelines.html)
- [OWASP Secrets Management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html)
- [Git Secrets](https://github.com/awslabs/git-secrets)

---

**¡Mantén tus credenciales seguras!** 🔐

Nunca compartas contraseñas por email, chat o repositorios públicos.
