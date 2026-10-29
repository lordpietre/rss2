/**
 * Pruebas de la seccion de alertas.
 *
 * Se comprueba lo que se pidio: al seleccionar una alerta se abre el listado
 * de noticias del concepto y cada noticia tiene las mismas acciones que en
 * la seccion de Inteligencia (guardar en favoritos y anadir a una lista).
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'

const mockGetAlertas = vi.fn()
const mockToggleFavorite = vi.fn()
const mockAddToList = vi.fn()
const mockCreateList = vi.fn()
const mockGetFavorites = vi.fn()
const mockGetLists = vi.fn()
const mockGetEntityNews = vi.fn()

vi.mock('../services/api', () => ({
  apiService: {
    getAlertas: (...args: unknown[]) => mockGetAlertas(...args),
    markAlertaRead: vi.fn(),
    dismissAlerta: vi.fn(),
    markAllAlertasRead: vi.fn(),
  },
  api: {
    get: (...args: unknown[]) => mockGetEntityNews(...args),
    post: vi.fn(),
  },
}))

vi.mock('../hooks/useApiFavorites', () => ({
  useApiFavorites: () => ({
    favorites: [],
    toggleFavorite: (...args: unknown[]) => mockToggleFavorite(...args),
    isFavorite: () => false,
  }),
  useApiLists: () => ({
    lists: [{ id: 7, name: 'Mi lista' }],
    createList: (...args: unknown[]) => mockCreateList(...args),
    addToList: (...args: unknown[]) => mockAddToList(...args),
  }),
}))

import { Alertas } from '../pages/Alertas'

const ALERTA = {
  id: 1,
  valor: 'Trump',
  tipo: 'persona',
  periodo: '2026-10-25 10:00',
  hits: 12,
  baseline: 1.5,
  ratio: 8,
  status: 'nueva',
  wiki_summary: '',
  wiki_url: '',
  image_path: '',
}

const NEWS = [
  {
    id: 'noticia-a',
    titulo: 'Primera noticia sobre Trump',
    resumen: 'Resumen de la primera',
    title_translated: 'Primera noticia sobre Trump (traducida)',
    summary_translated: 'Resumen traducido',
    url: 'https://example.org/a',
    fecha: '2026-10-25T09:00:00Z',
    lang: 'en',
    imagen_url: '',
  },
  {
    id: 'noticia-b',
    titulo: 'Segunda noticia sobre Trump',
    resumen: 'Resumen de la segunda',
    title_translated: 'Segunda noticia sobre Trump (traducida)',
    summary_translated: 'Resumen traducido',
    url: 'https://example.org/b',
    fecha: '2026-10-25T08:00:00Z',
    lang: 'en',
    imagen_url: '',
  },
]

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
        <Alertas />
      </QueryClientProvider>
    </MemoryRouter>
  )
}

describe('Alertas', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetAlertas.mockResolvedValue({ alertas: [ALERTA], nuevas: 1, total: 1 })
    // axios: el componente lee res.data.news
    mockGetEntityNews.mockResolvedValue({ data: { news: NEWS, total: NEWS.length } })
    mockAddToList.mockResolvedValue(undefined)
    mockCreateList.mockResolvedValue(42)
  })

  it('ofrece favoritos y anadir a lista en cada noticia del modal', async () => {
    renderPage()

    await waitFor(() => expect(screen.getByText('Trump')).toBeInTheDocument())

    // Abrir el detalle de noticias de la alerta
    fireEvent.click(screen.getByRole('button', { name: 'News' }))

    // Las dos noticias del concepto estan en el modal
    await waitFor(() =>
      expect(screen.getByText('Primera noticia sobre Trump (traducida)')).toBeInTheDocument()
    )
    expect(screen.getByText('Segunda noticia sobre Trump (traducida)')).toBeInTheDocument()

    // Y cada una tiene los dos botones
    const favButtons = screen.getAllByRole('button', { name: 'Añadir a favoritos' })
    const listButtons = screen.getAllByRole('button', { name: 'Añadir a lista' })
    expect(favButtons.length).toBe(NEWS.length)
    expect(listButtons.length).toBe(NEWS.length)
  })

  it('limita el listado a la hora del pico por defecto', async () => {
    // El mock responde distinto segun se mande el periodo: sin el, todas
    // las noticias del concepto; con el, solo las de esa hora.
    mockGetEntityNews.mockImplementation((url: string) => {
      const conPeriodo = String(url).includes('periodo=')
      const news = conPeriodo ? [NEWS[0]] : NEWS
      return Promise.resolve({ data: { news, total: news.length } })
    })

    renderPage()

    await waitFor(() => expect(screen.getByText('Trump')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'News' }))

    await waitFor(() =>
      expect(screen.getByText('Primera noticia sobre Trump (traducida)')).toBeInTheDocument()
    )

    // Se pidio el filtro por la hora del pico
    const url = mockGetEntityNews.mock.calls[0][0] as string
    expect(url).toContain('periodo=')
    expect(url).toContain('periodo=2026-10-25+10%3A00')

    // Y solo se lista la noticia de esa hora
    expect(screen.queryByText('Segunda noticia sobre Trump (traducida)')).not.toBeInTheDocument()
  })

  it('anade a favoritos la noticia pulsada', async () => {
    renderPage()

    await waitFor(() => expect(screen.getByText('Trump')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'News' }))

    await waitFor(() => expect(screen.getByText(/News about/)).toBeInTheDocument())
    fireEvent.click(screen.getAllByRole('button', { name: 'Añadir a favoritos' })[0])

    await waitFor(() => expect(mockToggleFavorite).toHaveBeenCalledTimes(1))
    expect(mockToggleFavorite.mock.calls[0][0].id).toBe('noticia-a')
  })

  it('abre el selector de listas al pulsar "anadir a lista"', async () => {
    renderPage()

    await waitFor(() => expect(screen.getByText('Trump')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'News' }))

    await waitFor(() => expect(screen.getByText(/News about/)).toBeInTheDocument())
    fireEvent.click(screen.getAllByRole('button', { name: 'Añadir a lista' })[0])

    // El modal de listas se abre y ofrece las listas existentes
    await waitFor(() => expect(screen.getByText('Mi lista')).toBeInTheDocument())
  })
})
