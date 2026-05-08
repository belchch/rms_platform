import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export function ProjectsPage() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Проекты</CardTitle>
        <CardDescription>Список проектов появится здесь.</CardDescription>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">Заглушка</p>
      </CardContent>
    </Card>
  );
}
