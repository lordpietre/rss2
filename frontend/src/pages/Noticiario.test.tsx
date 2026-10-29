/**
 * Pruebas de la pagina de noticiario por voz.
 *
 * Se simula la API: lo que se comprueba es el flujo de trabajo del editor
 * (buscar -> seleccionar -> crear) y que se envian los datos correctos.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const mockSearch = vi.fn()
const mockCreateBulletin = vi.fn()
const mockApproveBulletin = vi.fn()
const mockListBulletins = vi.fn()
const mockGetBulletin = vi.fn()
const mockGetBulletinBlob = vi.fn()
const mockSetBulletinStatus = vi.fn()
const mockGetCategories = vi.fn()
const mockGetCountries = vi.fn()
const mockListVoices = vi.fn()
const mockGetTrends = vi.fn()
const mockPrepareScripts = vi.fn()
const mockUpdateScript = vi.fn()

vi.mock('../services/api', () => ({
  apiService: {
    search: (...args: unknown[]) => mockSearch(...args),
    createTtsBulletin: (...args: unknown[]) => mockCreateBulletin(...args),
    approveTtsBulletin: (...args: unknown[]) => mockApproveBulletin(...args),
    listTtsBulletins: (...args: unknown[]) => mockListBulletins(...args),
    getTtsBulletin: (...args: unknown[]) => mockGetBulletin(...args),
    getTtsBulletinBlob: (...args: unknown[]) => mockGetBulletinBlob(...args),
    setTtsBulletinStatus: (...args: unknown[]) => mockSetBulletinStatus(...args),
    getCategories: (...args: unknown[]) => mockGetCategories(...args),
    getCountries: (...args: unknown[]) => mockGetCountries(...args),
    listTtsVoices: (...args: unknown[]) => mockListVoices(...args),
    getTtsTrends: (...args: unknown[]) => mockGetTrends(...args),
    prepareTtsScripts: (...args: unknown[]) => mockPrepareScripts(...args),
    updateTtsScript: (...args: unknown[]) => mockUpdateScript(...args),
  },
}))

// No se mockea AppContext a proposito: la app no monta AppProvider y la
// pagina no debe depender de useApp(). Si alguien lo reintrodujera, esta
// prueba lo detectaria.
vi.mock('../hooks/useApiFavorites', () => ({
  useApiLists: () => ({ lists: [], getListItems: vi.fn() }),
}))

import { Noticiario } from '../pages/Noticiario'

const NEWS = [
  {
    id: 'noticia-1',
    titulo: 'Primera noticia',
    resumen: 'Resumen de la primera noticia',
    title_translated: 'Primera noticia traducida',
    summary_translated: 'Resumen traducido de la primera',
    url: 'https://ejemplo.local/1',
    fecha: '2026-10-20T10:00:00Z',
    fuente_nombre: 'Medio A',
  },
  {
    id: 'noticia-2',
    titulo: 'Segunda noticia',
    resumen: 'Resumen de la segunda noticia',
    title_translated: 'Segunda noticia traducida',
    summary_translated: 'Resumen traducido de la segunda',
    url: 'https://ejemplo.local/2',
    fecha: '2026-10-20T11:00:00Z',
    fuente_nombre: 'Medio B',
  },
]

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <Noticiario />
    </QueryClientProvider>
  )
}

describe('Pagina de noticiario', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mockGetCategories.mockResolvedValue([])
    mockGetCountries.mockResolvedValue([])
    mockListBulletins.mockResolvedValue({ bulletins: [] })
    mockListVoices.mockResolvedValue({
      voices: [
        { id: 1, slug: 'es-base', display_name: 'Voz base en español', engine: 'melotts', language: 'es', quality_status: 'ok', is_builtin: true },
        { id: 2, slug: 'mi-voz', display_name: 'Mi voz', engine: 'openvoice', language: 'es', quality_status: 'ok', is_builtin: false },
      ],
    })
    mockGetTrends.mockResolvedValue({ clusters: [], topics: [], params: {} })
    mockGetBulletinBlob.mockResolvedValue(new Blob(['audio']))
    mockPrepareScripts.mockResolvedValue({
      scripts: [
        {
          script_id: 101,
          noticia_id: 'noticia-1',
          title: 'Primera noticia traducida',
          content: 'Texto de la primera noticia listo para locutar.',
          source_name: 'Medio A',
          source_url: 'https://ejemplo.local/1',
          status: 'draft',
          existing: false,
        },
      ],
    })
    mockUpdateScript.mockResolvedValue({
      id: 101,
      language: 'es',
      content: 'Texto corregido.',
      status: 'draft',
      created_at: '2026-10-24T10:00:00Z',
    })
    mockSearch.mockResolvedValue({
      news: NEWS,
      total: 2,
      page: 1,
      per_page: 20,
      total_pages: 1,
    })
    mockCreateBulletin.mockResolvedValue({
      id: 7,
      title: 'Noticiario de prueba',
      language: 'es',
      status: 'pending',
      item_count: 2,
      silence_ms: 800,
      created_at: '2026-10-24T10:00:00Z',
    })
  })

  it('muestra el buscador y el panel de seleccion vacio', () => {
    renderPage()

    expect(screen.getByText('Noticiario por voz')).toBeTruthy()
    expect(screen.getByPlaceholderText(/Palabras clave/i)).toBeTruthy()
    expect(screen.getByText(/Busca noticias y pulsa/i)).toBeTruthy()
  })

  it('busca con el termino escrito y muestra los resultados', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))

    await waitFor(() => expect(mockSearch).toHaveBeenCalled())
    expect(mockSearch.mock.calls[0][0].q).toBe('economia')

    expect(await screen.findByText('Primera noticia traducida')).toBeTruthy()
    expect(screen.getByText('Segunda noticia traducida')).toBeTruthy()
  })

  it('añade y quita noticias de la seleccion', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))
    await screen.findByText('Primera noticia traducida')

    const addButtons = screen.getAllByTitle('Añadir al noticiario')
    await user.click(addButtons[0])

    await waitFor(() => expect(screen.getByText('1 / 50')).toBeTruthy())

    // La noticia seleccionada aparece en el panel de la derecha.
    const panel = screen.getByText('Selección').closest('div')!.parentElement!
    expect(within(panel).getByText('Primera noticia traducida')).toBeTruthy()

    // Y se puede quitar.
    await user.click(screen.getByTitle('Quitar del noticiario'))
    await waitFor(() => expect(screen.getByText('0 / 50')).toBeTruthy())
  })

  it('crea el noticiario con las noticias seleccionadas en orden', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))
    await screen.findByText('Primera noticia traducida')

    const addButtons = screen.getAllByTitle('Añadir al noticiario')
    await user.click(addButtons[0])
    await user.click(addButtons[1])

    await waitFor(() => expect(screen.getByText('2 / 50')).toBeTruthy())

    await user.click(screen.getByRole('button', { name: /^procesar$/i }))

    await waitFor(() => expect(mockCreateBulletin).toHaveBeenCalled())
    const payload = mockCreateBulletin.mock.calls[0][0]
    // Se locutan los guiones preparados, en el orden de la seleccion.
    expect(payload.script_ids).toEqual([101])
    expect(payload.language).toBe('es')
    expect(payload.silence_ms).toBe(800)
    // La voz y la velocidad viajan al backend.
    expect(payload.voice).toBe('es-base')
    expect(payload.speed).toBe(1)
  })

  it('revisa el texto antes de locutarlo y deja corregirlo', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))
    await screen.findByText('Primera noticia traducida')
    await user.click(screen.getAllByTitle('Añadir al noticiario')[0])

    // Boton Revision: abre el texto completo, editable.
    await user.click(screen.getByRole('button', { name: /revisi/i }))
    await waitFor(() => expect(mockPrepareScripts).toHaveBeenCalled())
    expect(mockPrepareScripts.mock.calls[0][0].noticia_ids).toEqual(['noticia-1'])

    const textarea = await screen.findByLabelText(/texto de la noticia 1/i)
    expect((textarea as HTMLTextAreaElement).value).toContain('Texto de la primera noticia')

    // Se puede escribir cualquier modificacion.
    await user.clear(textarea)
    await user.type(textarea, 'Texto corregido por el editor.')
    await user.click(screen.getByRole('button', { name: /guardar/i }))

    await waitFor(() => expect(mockUpdateScript).toHaveBeenCalledWith(101, 'Texto corregido por el editor.'))
  })

  it('procesa con el texto revisado y lo guarda antes', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))
    await screen.findByText('Primera noticia traducida')
    await user.click(screen.getAllByTitle('Añadir al noticiario')[0])

    await user.click(screen.getByRole('button', { name: /revisi/i }))
    const textarea = await screen.findByLabelText(/texto de la noticia 1/i)
    await user.clear(textarea)
    await user.type(textarea, 'Texto corregido por el editor.')

    // Procesar desde el panel de revision.
    await user.click(screen.getByRole('button', { name: /procesar y generar el audio/i }))

    await waitFor(() => expect(mockUpdateScript).toHaveBeenCalled())
    await waitFor(() => expect(mockCreateBulletin).toHaveBeenCalled())
    expect(mockCreateBulletin.mock.calls[0][0].script_ids).toEqual([101])
  })

  it('permite elegir otra voz y otra velocidad', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))
    await screen.findByText('Primera noticia traducida')
    await user.click(screen.getAllByTitle('Añadir al noticiario')[0])

    // Selector de voz con las voces disponibles.
    const voiceSelect = await screen.findByLabelText(/voz/i)
    await user.selectOptions(voiceSelect, 'mi-voz')

    // Velocidad (input range: se cambia con fireEvent).
    const speed = screen.getByLabelText(/velocidad/i)
    fireEvent.change(speed, { target: { value: '1.25' } })

    await user.click(screen.getByRole('button', { name: /procesar/i }))

    await waitFor(() => expect(mockCreateBulletin).toHaveBeenCalled())
    const payload = mockCreateBulletin.mock.calls[0][0]
    expect(payload.voice).toBe('mi-voz')
    expect(payload.speed).toBeCloseTo(1.25, 2)
  })

  it('pasa los filtros pensados para el informativo a la busqueda', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))

    await waitFor(() => expect(mockSearch).toHaveBeenCalled())
    const params = mockSearch.mock.calls[0][0]
    expect(params.desde).toBe('7d')
    expect(params.orden).toBe('reciente')
    expect(params.min_chars).toBe(150)
    expect(params.dedup).toBe('cluster')
  })

  it('ofrece el modo tendencias con las historias agrupadas', async () => {
    mockGetTrends.mockResolvedValue({
      clusters: [
        {
          id: 3,
          title: 'Trump y los markets',
          importance: 'breaking',
          popularity: 21,
          news_count: 21,
          representative: {
            id: 'noticia-trend',
            title: 'Titular de la tendencia',
            summary: 'Resumen',
            content_length: 800,
            source: 'EFE',
            url: 'https://ejemplo.local/t',
            cluster_id: 3,
            cluster_popularity: 21,
          },
        },
      ],
      topics: [{ id: 1, name: 'Política', news: 120, score: 0.8 }],
      params: {},
    })

    const user = userEvent.setup()
    renderPage()

    await user.click(screen.getByRole('button', { name: /por tendencia/i }))
    await waitFor(() => expect(mockGetTrends).toHaveBeenCalled())

    expect(await screen.findByText(/Titular de la tendencia/)).toBeTruthy()
    expect(screen.getByText('Política · 120')).toBeTruthy()

    // Y se puede añadir la noticia principal de la historia.
    await user.click(screen.getByRole('button', { name: /añadir la noticia principal/i }))
    await waitFor(() => expect(screen.getByText('1 / 50')).toBeTruthy())
  })

  it('no procesa ni revisa sin noticias seleccionadas', async () => {
    const user = userEvent.setup()
    renderPage()

    const process = screen.getByRole('button', { name: /^procesar$/i })
    const review = screen.getByRole('button', { name: /revisi/i })
    expect((process as HTMLButtonElement).disabled).toBe(true)
    expect((review as HTMLButtonElement).disabled).toBe(true)
  })

  it('aprueba el contenido del noticiario en curso', async () => {
    mockGetBulletin.mockResolvedValue({
      id: 7,
      title: 'Noticiario de prueba',
      language: 'es',
      status: 'pending',
      item_count: 2,
      silence_ms: 800,
      created_at: '2026-10-24T10:00:00Z',
      items: [
        { id: 1, position: 1, script_id: 1, source_title: 'Primera noticia traducida' },
        { id: 2, position: 2, script_id: 2, source_title: 'Segunda noticia traducida' },
      ],
    })
    mockApproveBulletin.mockResolvedValue({ bulletin: {}, queued: 2 })

    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))
    await screen.findByText('Primera noticia traducida')

    const addButtons = screen.getAllByTitle('Añadir al noticiario')
    await user.click(addButtons[0])
    await user.click(screen.getByRole('button', { name: /procesar/i }))

    const approve = await screen.findByRole('button', { name: /aprobar y generar/i })
    await user.click(approve)

    await waitFor(() => expect(mockApproveBulletin).toHaveBeenCalledWith(7))
  })

  it('lista los noticiarios anteriores', async () => {
    mockListBulletins.mockResolvedValue({
      bulletins: [
        {
          id: 3,
          title: 'Edicion del lunes',
          language: 'es',
          status: 'ready',
          item_count: 5,
          silence_ms: 800,
          duration_seconds: 394.8,
          created_at: '2026-10-23T10:00:00Z',
        },
      ],
    })

    renderPage()

    expect(await screen.findByText('Edicion del lunes')).toBeTruthy()
    expect(screen.getByText('6:35')).toBeTruthy() // 394.8 s
  })

  it('renderiza sin AppProvider y detecta al admin desde localStorage', async () => {
    // La app no monta AppProvider: la pagina no debe usar useApp().
    localStorage.setItem('user', JSON.stringify({ id: 1, is_admin: true }))
    mockCreateBulletin.mockResolvedValue({
      id: 7,
      title: 'Noticiario listo',
      language: 'es',
      status: 'ready',
      item_count: 1,
      silence_ms: 800,
      audio_path: 'bulletins/bulletin-7.wav',
      duration_seconds: 42,
      created_at: '2026-10-24T10:00:00Z',
    })

    mockGetBulletin.mockResolvedValue({
      id: 7,
      title: 'Noticiario listo',
      language: 'es',
      status: 'ready',
      item_count: 1,
      silence_ms: 800,
      audio_path: 'bulletins/bulletin-7.wav',
      duration_seconds: 42,
      created_at: '2026-10-24T10:00:00Z',
    })

    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))
    await screen.findByText('Primera noticia traducida')
    await user.click(screen.getAllByTitle('Añadir al noticiario')[0])
    await user.click(screen.getByRole('button', { name: /procesar/i }))

    // Con permisos de admin aparece el boton de aprobacion final.
    expect(await screen.findByRole('button', { name: /^aprobar$/i })).toBeTruthy()
  })

  it('no muestra la aprobacion final a un usuario normal', async () => {
    localStorage.setItem('user', JSON.stringify({ id: 2, is_admin: false }))
    mockCreateBulletin.mockResolvedValue({
      id: 8,
      title: 'Noticiario listo',
      language: 'es',
      status: 'ready',
      item_count: 1,
      silence_ms: 800,
      audio_path: 'bulletins/bulletin-8.wav',
      duration_seconds: 42,
      created_at: '2026-10-24T10:00:00Z',
    })

    mockGetBulletin.mockResolvedValue({
      id: 8,
      title: 'Noticiario listo',
      language: 'es',
      status: 'ready',
      item_count: 1,
      silence_ms: 800,
      audio_path: 'bulletins/bulletin-8.wav',
      duration_seconds: 42,
      created_at: '2026-10-24T10:00:00Z',
    })

    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByPlaceholderText(/Palabras clave/i), 'economia')
    await user.click(screen.getByRole('button', { name: /buscar/i }))
    await screen.findByText('Primera noticia traducida')
    await user.click(screen.getAllByTitle('Añadir al noticiario')[0])
    await user.click(screen.getByRole('button', { name: /procesar/i }))

    await screen.findByText('Noticiario listo')
    expect(screen.queryByRole('button', { name: /^aprobar$/i })).toBeNull()
    expect(screen.getByText(/un administrador debe aprobar/i)).toBeTruthy()
  })
})
