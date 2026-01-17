import { defineStore } from 'pinia'
import { get, post, put, setToken, getToken } from '@/lib/http'

interface BackendUser {
	id: number
	username: string
	nickname: string
	email?: string | null
	avatar_url?: string
	role: string
	created_at: string
}

interface LoginResponse {
	token: string
	user: BackendUser
}

interface UserInfoResponse {
	user: BackendUser
}

export const useUserStore = defineStore('user', {
	state: () => ({
		token: getToken() as string | null,
		currentUser: null as BackendUser | null,
	}),
	getters: {
		isLoggedIn(state): boolean {
			return !!state.token
		},
		displayName(state): string {
			if (state.currentUser?.nickname && state.currentUser.nickname.length > 0) {
				return state.currentUser.nickname
			}
			return state.currentUser?.username ?? ''
		},
	},
	actions: {
		setAuth(token: string | null, user: BackendUser | null) {
			this.token = token
			this.currentUser = user
			setToken(token)
		},
		async login(username: string, password: string) {
			const resp = await post<LoginResponse>('/auth/login', {
				username,
				password,
			})
			this.setAuth(resp.token, resp.user)
		},
		async fetchUserInfo() {
			if (!this.token) {
				return
			}
			const resp = await get<UserInfoResponse>('/user/info')
			this.currentUser = resp.user
		},
		logout() {
			this.setAuth(null, null)
		},
		async updateProfileWithAvatar(payload: { nickname?: string; avatarFile?: File | null }) {
			const formData = new FormData()
			if (payload.nickname) {
				formData.append('nickname', payload.nickname)
			}
			if (payload.avatarFile) {
				formData.append('avatar', payload.avatarFile)
			}
			const resp = await put<{ message: string; avatar_url?: string }>(
				'/user/profile',
				formData,
				{
					headers: {
						'Content-Type': 'multipart/form-data',
					},
				},
			)
			await this.fetchUserInfo()
			return resp
		},
	},
})
