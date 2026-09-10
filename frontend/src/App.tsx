import { useEffect } from 'react'
import { useApp } from './store'
import TopBar from './components/common/TopBar'
import IconRail from './components/Sidebar/IconRail'
import NavSidebar from './components/Sidebar/NavSidebar'
import ChatView from './components/Chat/ChatView'
import LibraryView from './components/Library/LibraryView'
import WorkspaceView from './components/Workspace/WorkspaceView'
import AvatarView from './components/Avatar/AvatarView'
import InboxView from './components/Inbox/InboxView'
import AdminView from './components/Admin/AdminView'
import RightPanel from './components/right/RightPanel'
import KeyGate from './components/KeyGate'

export default function App() {
  const view = useApp((s) => s.view)
  const key = useApp((s) => s.key)
  const verifyKey = useApp((s) => s.verifyKey)
  const refreshConversations = useApp((s) => s.refreshConversations)
  const refreshInbox = useApp((s) => s.refreshInbox)
  const refreshAvatar = useApp((s) => s.refreshAvatar)

  useEffect(() => {
    verifyKey()
    refreshConversations()
    refreshInbox()
    refreshAvatar()
  }, [verifyKey, refreshConversations, refreshInbox, refreshAvatar])

  if (!key) return <KeyGate />

  return (
    <div className="flex flex-col h-screen bg-ink-950 text-neutral-200">
      <TopBar />
      <div className="flex flex-1 min-h-0">
        <IconRail />
        <NavSidebar />
        <main className="flex-1 min-w-0 flex flex-col">
          {view === 'chat' && <ChatView />}
          {view === 'library' && <LibraryView />}
          {view === 'workspace' && <WorkspaceView />}
          {view === 'avatar' && <AvatarView />}
          {view === 'inbox' && <InboxView />}
          {view === 'admin' && <AdminView />}
        </main>
        <RightPanel />
      </div>
    </div>
  )
}
