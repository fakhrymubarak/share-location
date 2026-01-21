import type { JSX } from 'solid-js'

type LayoutProps = {
    children: JSX.Element
}

export function Layout(props: LayoutProps) {
    return (
        <div class="page-layout">
            <div class="backdrop" />
            <main class="main-content">
                {props.children}
            </main>
        </div>
    )
}
