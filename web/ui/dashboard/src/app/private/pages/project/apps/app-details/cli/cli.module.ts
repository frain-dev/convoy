import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CliComponent } from './cli.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { PipesModule } from 'src/app/pipes/pipes.module';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';



@NgModule({
  declarations: [CliComponent],
  imports: [
    CommonModule, CardComponent, ButtonComponent, EmptyStateComponent, TagComponent, SkeletonLoaderComponent, PipesModule
  ],
  exports: [CliComponent]
})
export class CliModule { }
