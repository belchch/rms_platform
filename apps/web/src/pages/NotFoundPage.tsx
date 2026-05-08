import { Link } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";

export function NotFoundPage() {
  return (
    <div className="flex min-h-[50vh] flex-col items-center justify-center gap-4 text-center">
      <p className="text-6xl font-semibold text-muted-foreground">404</p>
      <h1 className="text-xl font-medium">Страница не найдена</h1>
      <Button asChild>
        <Link to="/">На главную</Link>
      </Button>
    </div>
  );
}
