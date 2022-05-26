import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SourcesComponent } from './sources.component';
import { Routes, RouterModule } from '@angular/router';
import { CreateSourceModule } from 'src/app/private/components/create-source/create-source.module';

const routes: Routes = [{ path: '', component: SourcesComponent }];

@NgModule({
	declarations: [SourcesComponent],
	imports: [CommonModule, RouterModule.forChild(routes), CreateSourceModule]
})
export class SourcesModule {}
