import request from '../utils/request'
import type { WantedVO } from './types'

export function listWanteds(params: Record<string, unknown>) {
  return request.get('/wanteds', { params })
}

export function getWanted(id: number) {
  return request.get(`/wanteds/${id}`)
}

export function createWanted(data: Record<string, unknown>) {
  return request.post('/wanteds', data)
}

export function closeWanted(id: number) {
  return request.post(`/wanteds/${id}/close`)
}

export function myWanteds(params: Record<string, unknown>) {
  return request.get('/wanteds/mine', { params })
}

export type { WantedVO }
