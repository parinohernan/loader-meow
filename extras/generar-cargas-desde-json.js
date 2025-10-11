const { createClient } = require('@supabase/supabase-js');
const fs = require('fs');
const path = require('path');

// =====================================================
// CONFIGURACIÓN DE SUPABASE
// =====================================================
const supabaseUrl = 'https://ikiusmdtltakhmmlljsp.supabase.co';
const supabaseAnonKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImlraXVzbWR0bHRha2htbWxsanNwIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MzQ2MjkyMzEsImV4cCI6MjA1MDIwNTIzMX0.q6NMMUK2ONGFs-b10XZySVlQiCXSLsjZbtBZyUTiVjc';

const supabase = createClient(supabaseUrl, supabaseAnonKey);

// =====================================================
// DATOS DE REFERENCIA
// =====================================================

// Materiales disponibles
const MATERIALES = [
    { id: 'b93fcec6-b173-47d4-be39-52d272bc8a87', nombre: 'Agroquímicos' },
    { id: '97ca010a-6375-40d6-880e-051ba3818516', nombre: 'Alimentos y bebidas' },
    { id: '220193b8-bafe-476d-a225-433b567db256', nombre: 'Fertilizante' },
    { id: 'b181d3b8-f92c-44fd-9334-c34041ef29df', nombre: 'Ganado' },
    { id: 'bdb09420-ef80-4de0-a038-03285d48fb92', nombre: 'Girasol' },
    { id: '6def5e3b-358d-46e5-9170-8e42a2c97d23', nombre: 'Maiz' },
    { id: '4e9efe3d-8eb6-4600-96dd-eb35cbad8699', nombre: 'Maquinarias' },
    { id: '49ebf50f-d37a-446c-927c-f463fda953e0', nombre: 'Materiales construcción' },
    { id: '8cd407f6-297e-4730-a1d6-15a2ac485809', nombre: 'Otras cargas generales' },
    { id: 'c921caf8-5e2b-4fdb-9190-d7fe624771bf', nombre: 'Otros cultivos' },
    { id: '176bf83f-3109-431d-8a35-1d157ae4d91f', nombre: 'Refrigerados' },
    { id: '4edee3cb-7308-4d1b-96e7-a378052004e7', nombre: 'Soja' },
    { id: '04ba66a5-6a87-4243-b8ed-45baf6cfc2e8', nombre: 'Trigo' }
];

// Presentaciones disponibles
const PRESENTACIONES = [
    { id: 'ca7cf082-837c-4c14-b2ad-c85f0821d86c', nombre: 'Big Bag' },
    { id: 'e676ca36-8a96-4338-9a41-2692c18664f5', nombre: 'Bolsa' },
    { id: '3923f3da-eb7d-4438-8fcd-74d53891c392', nombre: 'Granel' },
    { id: '510db5c8-eb5f-4ef1-b23a-96d4e4869f2d', nombre: 'Otros' },
    { id: '234a739b-6666-4595-a8df-51e840c09599', nombre: 'Pallet' }
];

// Formas de pago disponibles
const FORMAS_PAGO = [
    { id: '48c0c41f-ed88-4b3a-b06d-9a1f03131fe8', nombre: 'Cheque' },
    { id: '692684a5-9103-4257-a3e3-6486f907177a', nombre: 'E-check' },
    { id: 'c96c6cd8-8742-4a8c-9df6-18554a7c87af', nombre: 'Efectivo' },
    { id: 'e0f74bf6-2886-44da-9469-c68ffaf53e4f', nombre: 'Otros' },
    { id: '7b998228-2121-465b-9721-679a320e50ae', nombre: 'Transferencia' }
];

// Tipos de equipo disponibles
const TIPOS_EQUIPO = [
    { id: '85bf5951-50a7-4abc-af6e-ea3b9550d97d', nombre: 'Batea' },
    { id: '8fa614ad-af82-4909-b0ff-b1d288ea97a3', nombre: 'Camioneta' },
    { id: '1933f25d-eb8e-43cf-b2e8-5224ab6a4ef2', nombre: 'CamionJaula' },
    { id: '779ba2a1-f4e3-4121-be59-3e1cdd2c6da8', nombre: 'Carreton' },
    { id: 'a16bdd90-df15-4adf-8cc4-7a74ad375ffd', nombre: 'Chasis y Acoplado' },
    { id: '9eb2b303-5c92-45ae-8120-4cc40dd3fa49', nombre: 'Furgon' },
    { id: 'e1c0cc7d-27fb-4206-9fe3-280ffc40d742', nombre: 'Otros' },
    { id: 'be085c4d-f6a5-4f36-b869-9ec606bef794', nombre: 'Semi' },
    { id: '5939b8d1-71d7-4e37-851b-db388856945e', nombre: 'Tolva' }
];

// =====================================================
// FUNCIONES AUXILIARES
// =====================================================

/**
 * Buscar elemento por nombre en array
 */
function buscarPorNombre(array, nombre) {
    if (!nombre) return null;
    return array.find(item => 
        item.nombre.toLowerCase() === nombre.toLowerCase()
    );
}

/**
 * Limpiar valores null del JSON
 */
function limpiarValoresNull(obj) {
    const objLimpio = {};
    for (const [key, value] of Object.entries(obj)) {
        if (value !== null && value !== undefined) {
            objLimpio[key] = value;
        }
    }
    return objLimpio;
}

/**
 * Validar teléfono argentino
 */
function validarTelefono(telefono) {
    if (!telefono) return null;
    
    // Remover espacios y caracteres especiales
    const limpio = telefono.replace(/[\s\-\(\)]/g, '');
    
    // Verificar si empieza con +54
    if (limpio.startsWith('+54')) {
        return limpio;
    }
    
    // Verificar si empieza con 54
    if (limpio.startsWith('54')) {
        return '+' + limpio;
    }
    
    // Verificar si empieza con 9 (celular argentino)
    if (limpio.startsWith('9')) {
        return '+54' + limpio;
    }
    
    // Verificar si empieza con 11, 15, 351, etc. (códigos de área)
    const codigosArea = ['11', '15', '351', '341', '381', '387', '388', '299', '280', '290'];
    for (const codigo of codigosArea) {
        if (limpio.startsWith(codigo)) {
            return '+54' + limpio;
        }
    }
    
    return null;
}

/**
 * Validar email
 */
function validarEmail(email) {
    if (!email) return true; // Email es opcional
    const regex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return regex.test(email);
}

/**
 * Validar peso
 */
function validarPeso(peso) {
    if (!peso) return false;
    const numero = parseFloat(peso);
    return !isNaN(numero) && numero > 0;
}

/**
 * Validar precio
 */
function validarPrecio(precio) {
    if (!precio) return true; // Precio es opcional
    const numero = parseFloat(precio);
    return !isNaN(numero) && numero >= 0;
}

/**
 * Formatear fecha
 */
function formatearFecha(fecha) {
    if (!fecha) return new Date().toISOString().split('T')[0];
    
    // Si ya está en formato YYYY-MM-DD
    if (fecha.match(/^\d{4}-\d{2}-\d{2}$/)) {
        return fecha;
    }
    
    // Si está en formato DD/MM/YYYY
    if (fecha.match(/^\d{2}\/\d{2}\/\d{4}$/)) {
        const partes = fecha.split('/');
        const [dia, mes, año] = partes;
        return `${año}-${mes.padStart(2, '0')}-${dia.padStart(2, '0')}`;
    }
    
    return new Date().toISOString().split('T')[0];
}

/**
 * Obtener coordenadas de Google Geocoding
 */
async function obtenerCoordenadas(direccion) {
    try {
        const GOOGLE_API_KEY = "AIzaSyASe9Id-6Dr6lxr5mCb7O3l2HlmNrY-mRU";
        const formattedAddress = encodeURIComponent(direccion.trim());
        
        // Agregar components=country:AR para forzar resultados en Argentina
        // Agregar region=AR para dar bias a resultados argentinos
        const url = `https://maps.googleapis.com/maps/api/geocode/json?address=${formattedAddress}&components=country:AR&region=AR&key=${GOOGLE_API_KEY}&language=es`;
        
        console.log(`🗺️ Geocodificando: ${direccion}`);
        
        const response = await fetch(url);
        const data = await response.json();
        
        if (data.status === 'OK' && data.results.length > 0) {
            const result = data.results[0];
            const location = result.geometry.location;
            
            // Log de la dirección formateada que devolvió Google
            console.log(`✅ Coordenadas obtenidas: ${location.lat}, ${location.lng}`);
            console.log(`📍 Dirección detectada: ${result.formatted_address}`);
            
            return { lat: location.lat, lng: location.lng };
        } else {
            console.error(`❌ Error geocodificando: ${direccion}`, data.status);
            return null;
        }
    } catch (error) {
        console.error(`❌ Error en geocodificación:`, error);
        return null;
    }
}

/**
 * Obtener o crear ubicación
 */
async function obtenerOCrearUbicacion(direccion) {
    try {
        // 1. Buscar si ya existe
        const { data: existing, error: searchError } = await supabase
            .from('ubicaciones')
            .select('id')
            .eq('direccion', direccion)
            .limit(1)
            .maybeSingle();

        if (searchError) {
            console.error('❌ Error buscando ubicación:', searchError);
            return null;
        }

        if (existing) {
            console.log(`✅ Ubicación encontrada: ${direccion} (ID: ${existing.id})`);
            return existing.id;
        }

        // 2. Obtener coordenadas
        const coordenadas = await obtenerCoordenadas(direccion);
        if (!coordenadas) {
            console.error(`❌ No se pudieron obtener coordenadas para: ${direccion}`);
            return null;
        }

        // 3. Crear nueva ubicación
        const { data: newUbicacion, error: insertError } = await supabase
            .from('ubicaciones')
            .insert({
                direccion: direccion,
                lat: coordenadas.lat,
                lng: coordenadas.lng
            })
            .select('id')
            .single();

        if (insertError) {
            console.error('❌ Error creando ubicación:', insertError);
            return null;
        }

        console.log(`✅ Nueva ubicación creada: ${direccion} (ID: ${newUbicacion.id})`);
        return newUbicacion.id;
    } catch (error) {
        console.error('❌ Error en obtenerOCrearUbicacion:', error);
        return null;
    }
}

/**
 * Obtener usuarios dadores disponibles
 */
async function obtenerDadores() {
    try {
        const { data: dadores, error } = await supabase
            .from('usuarios')
            .select('id, nombre, email')
            .eq('rol_id', 2)
            .order('nombre');

        if (error) {
            console.error('❌ Error obteniendo dadores:', error);
            return [];
        }

        return dadores;
    } catch (error) {
        console.error('❌ Error obteniendo dadores:', error);
        return [];
    }
}

/**
 * Validar datos de una carga
 */
function validarCarga(carga, index) {
    const errores = [];
    
    // Campos obligatorios
    if (!carga.material) {
        errores.push('Material es obligatorio');
    } else {
        const material = buscarPorNombre(MATERIALES, carga.material);
        if (!material) {
            errores.push(`Material "${carga.material}" no válido`);
        }
    }
    
    // Usar tipoCarga en lugar de presentacion
    const tipoCarga = carga.tipoCarga || carga.presentacion;
    if (!tipoCarga) {
        errores.push('Tipo de carga es obligatorio');
    } else {
        const presentacion = buscarPorNombre(PRESENTACIONES, tipoCarga);
        if (!presentacion) {
            errores.push(`Tipo de carga "${tipoCarga}" no válido`);
        }
    }
    
    if (!validarPeso(carga.peso)) {
        errores.push('Peso es obligatorio y debe ser un número mayor a 0');
    }
    
    if (!carga.tipoEquipo) {
        errores.push('Tipo de equipo es obligatorio');
    } else {
        const tipoEquipo = buscarPorNombre(TIPOS_EQUIPO, carga.tipoEquipo);
        if (!tipoEquipo) {
            errores.push(`Tipo de equipo "${carga.tipoEquipo}" no válido`);
        }
    }
    
    if (!carga.localidadCarga) {
        errores.push('Localidad de carga es obligatoria');
    }
    
    if (!carga.localidadDescarga) {
        errores.push('Localidad de descarga es obligatoria');
    }
    
    if (!carga.fechaCarga) {
        errores.push('Fecha de carga es obligatoria');
    }
    
    if (!carga.fechaDescarga) {
        errores.push('Fecha de descarga es obligatoria');
    }
    
    if (!carga.telefono) {
        errores.push('Teléfono es obligatorio');
    } else {
        const telefonoValidado = validarTelefono(carga.telefono);
        if (!telefonoValidado) {
            errores.push('Teléfono no tiene formato válido');
        }
    }
    
    // Campos opcionales con validación
    if (carga.correo && !validarEmail(carga.correo)) {
        errores.push('Email no tiene formato válido');
    }
    
    if (carga.precio && !validarPrecio(carga.precio)) {
        errores.push('Precio debe ser un número mayor o igual a 0');
    }
    
    if (carga.formaDePago) {
        const formaDePago = buscarPorNombre(FORMAS_PAGO, carga.formaDePago);
        if (!formaDePago) {
            errores.push(`Forma de pago "${carga.formaDePago}" no válida`);
        }
    }
    
    // Validar campos específicos del formato de archivodecargas.json
    if (carga.confianza && (carga.confianza < 0 || carga.confianza > 100)) {
        errores.push('Confianza debe estar entre 0 y 100');
    }
    
    return errores;
}

/**
 * Crear carga en la base de datos
 */
async function crearCarga(datosCarga, index) {
    try {
        console.log(`\n📦 Creando carga ${index + 1}...`);

        // Validar datos
        const errores = validarCarga(datosCarga, index);
        if (errores.length > 0) {
            console.error(`❌ Errores en carga ${index + 1}:`, errores);
            return { success: false, errores };
        }

        // Obtener ubicaciones
        const ubicacionInicialId = await obtenerOCrearUbicacion(datosCarga.localidadCarga);
        const ubicacionFinalId = await obtenerOCrearUbicacion(datosCarga.localidadDescarga);

        if (!ubicacionInicialId || !ubicacionFinalId) {
            throw new Error('No se pudieron obtener las ubicaciones');
        }

        // Obtener dadores
        // const dadores = await obtenerDadores();
        // if (dadores.length === 0) {
        //     throw new Error('No hay dadores disponibles');
        // }

        // // Seleccionar dador (por ahora el primero)
        // const dador = dadores[0];

        // Usar dador específico configurado    

        const dador = {
            id: '20d060b6-33b5-4222-a039-a3e603d979be',
            nombre: 'Carica Cargador Automatico',
            email: 'carga@carina.com.ar'
        };
        // Buscar IDs de los elementos seleccionados
        const material = buscarPorNombre(MATERIALES, datosCarga.material);
        const tipoCarga = datosCarga.tipoCarga || datosCarga.presentacion;
        const presentacion = buscarPorNombre(PRESENTACIONES, tipoCarga);
        const tipoEquipo = buscarPorNombre(TIPOS_EQUIPO, datosCarga.tipoEquipo);
        const formaDePago = buscarPorNombre(FORMAS_PAGO, datosCarga.formaDePago) || FORMAS_PAGO[2]; // Efectivo por defecto

        // Mapear datos a la estructura de la BD
        const cargaData = {
            dador_id: dador.id,
            peso: datosCarga.peso.toString(),
            ubicacioninicial_id: ubicacionInicialId,
            ubicacionfinal_id: ubicacionFinalId,
            telefonodador: validarTelefono(datosCarga.telefono),
            puntoreferencia: datosCarga.puntoReferencia || " ",
            material_id: material.id,
            presentacion_id: presentacion.id,
            valorviaje: datosCarga.precio || "0",
            pagopor: datosCarga.pagoPor || 'Otros',
            otropagopor: null,
            fechacarga: formatearFecha(datosCarga.fechaCarga),
            fechadescarga: formatearFecha(datosCarga.fechaDescarga),
            formadepago_id: formaDePago.id,
            email: datosCarga.correo || " ",
            tipo_equipo: tipoEquipo.id,
            observaciones: datosCarga.observaciones || " "
        };

        // Crear la carga
        const { data: newCarga, error: cargaError } = await supabase
            .from('cargas')
            .insert(cargaData)
            .select('id');

        if (cargaError) {
            throw new Error(`Error creando carga: ${cargaError.message}`);
        }

        console.log(`✅ Carga ${index + 1} creada exitosamente (ID: ${newCarga[0].id})`);
        return { success: true, cargaId: newCarga[0].id };
    } catch (error) {
        console.error(`❌ Error creando carga ${index + 1}:`, error.message);
        return { success: false, error: error.message };
    }
}

/**
 * Procesar array de cargas desde JSON
 */
async function procesarCargasDesdeJSON(cargas) {
    console.log('🚛 PROCESANDO CARGAS DESDE JSON');
    console.log('═══════════════════════════════════════════════════════════');
    console.log(`📊 Total de cargas a procesar: ${cargas.length}\n`);

    let exitosas = 0;
    let fallidas = 0;
    const resultados = [];

    for (let i = 0; i < cargas.length; i++) {
        const carga = limpiarValoresNull(cargas[i]);
        console.log(`\n📋 Procesando carga ${i + 1}/${cargas.length}`);
        console.log(`📦 Material: ${carga.material || 'No especificado'}`);
        console.log(`📍 Ruta: ${carga.localidadCarga || 'No especificado'} → ${carga.localidadDescarga || 'No especificado'}`);
        if (carga.confianza) {
            console.log(`🤖 Confianza de IA: ${carga.confianza}%`);
        }
        if (carga.errores && carga.errores.length > 0) {
            console.log(`⚠️ Errores detectados: ${carga.errores.join(', ')}`);
        }

        const resultado = await crearCarga(carga, i);
        resultados.push({
            index: i + 1,
            datos: carga,
            resultado: resultado
        });

        if (resultado.success) {
            exitosas++;
        } else {
            fallidas++;
        }

        // Pequeña pausa entre cargas para no sobrecargar la BD
        if (i < cargas.length - 1) {
            await new Promise(resolve => setTimeout(resolve, 1000));
        }
    }

    // Mostrar resumen final
    console.log('\n🎉 PROCESAMIENTO COMPLETADO');
    console.log('═'.repeat(50));
    console.log(`✅ Cargas creadas exitosamente: ${exitosas}`);
    console.log(`❌ Cargas fallidas: ${fallidas}`);
    console.log(`📊 Total procesadas: ${exitosas + fallidas}`);
    console.log(`📈 Tasa de éxito: ${((exitosas / cargas.length) * 100).toFixed(1)}%`);

    // Mostrar detalles de errores
    if (fallidas > 0) {
        console.log('\n❌ DETALLES DE ERRORES:');
        console.log('─'.repeat(50));
        resultados.forEach(resultado => {
            if (!resultado.resultado.success) {
                console.log(`Carga ${resultado.index}: ${resultado.resultado.error || resultado.resultado.errores?.join(', ')}`);
            }
        });
    }
    
    // Mostrar estadísticas de confianza de IA
    const cargasConConfianza = cargas.filter(c => c.confianza);
    if (cargasConConfianza.length > 0) {
        const confianzaPromedio = cargasConConfianza.reduce((sum, c) => sum + c.confianza, 0) / cargasConConfianza.length;
        console.log('\n🤖 ESTADÍSTICAS DE IA:');
        console.log('─'.repeat(50));
        console.log(`📊 Confianza promedio: ${confianzaPromedio.toFixed(1)}%`);
        console.log(`🔢 Cargas con confianza: ${cargasConConfianza.length}/${cargas.length}`);
    }

    return {
        exitosas,
        fallidas,
        total: cargas.length,
        resultados
    };
}

/**
 * Cargar JSON desde archivo
 */
function cargarJSONDesdeArchivo(rutaArchivo) {
    try {
        const rutaCompleta = path.resolve(rutaArchivo);
        const contenido = fs.readFileSync(rutaCompleta, 'utf8');
        const json = JSON.parse(contenido);
        
        if (!Array.isArray(json)) {
            throw new Error('El JSON debe ser un array de cargas');
        }
        
        return json;
    } catch (error) {
        console.error('❌ Error cargando JSON:', error.message);
        throw error;
    }
}

// =====================================================
// FUNCIÓN PRINCIPAL
// =====================================================

async function main() {
    const args = process.argv.slice(2);
    
    if (args.length === 0) {
        console.log('📝 Uso: node generar-cargas-desde-json.js <ruta-al-archivo-json>');
        console.log('📝 Ejemplo: node generar-cargas-desde-json.js cargas.json');
        console.log('📝 Ejemplo: node generar-cargas-desde-json.js ./datos/cargas.json');
        return;
    }

    const rutaArchivo = args[0];
    
    try {
        // Cargar JSON
        console.log(`📁 Cargando archivo: ${rutaArchivo}`);
        const cargas = cargarJSONDesdeArchivo(rutaArchivo);
        
        // Procesar cargas
        const resultado = await procesarCargasDesdeJSON(cargas);
        
        // Mostrar resultado final
        console.log('\n📊 RESULTADO FINAL:');
        console.log(JSON.stringify(resultado, null, 2));
        
    } catch (error) {
        console.error('❌ Error en la ejecución:', error.message);
        process.exit(1);
    }
}

// =====================================================
// EXPORTAR FUNCIONES
// =====================================================

module.exports = {
    procesarCargasDesdeJSON,
    crearCarga,
    validarCarga,
    obtenerOCrearUbicacion
};

// =====================================================
// EJECUTAR SI ES EL ARCHIVO PRINCIPAL
// =====================================================

if (require.main === module) {
    main().catch(error => {
        console.error('❌ Error en la ejecución:', error);
        process.exit(1);
    });
}

