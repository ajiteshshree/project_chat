import { useState } from 'react'
import Picker from '@emoji-mart/react'
import data from '@emoji-mart/data'

export default function EmojiButton({ onPick }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="emoji-wrap">
      <button className="icon-btn" title="Emoji" onClick={() => setOpen((o) => !o)}>
        🙂
      </button>
      {open && (
        <>
          <div className="emoji-backdrop" onClick={() => setOpen(false)} />
          <div className="emoji-pop">
            <Picker
              data={data}
              onEmojiSelect={(e) => {
                onPick(e.native)
                setOpen(false)
              }}
              theme="dark"
              previewPosition="none"
              skinTonePosition="none"
            />
          </div>
        </>
      )}
    </div>
  )
}